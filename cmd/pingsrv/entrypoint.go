package main

// This file contains the common serving entrypoint code used by both test and production scenarios.

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"

	"github.com/go-stack/stack"
	"github.com/karlmutch/go-service/pkg/components"
	"github.com/karlmutch/go-service/pkg/runtime"
	"github.com/karlmutch/go-service/pkg/server"

	"github.com/karlmutch/kv"
)

type serverOpts struct {
	serviceID string
	cfgHost   string

	ipPort string

	certPemFn string
	certKeyFn string

	cfgNamespace string
	cfgConfigMap string

	prometheusAddr    string
	prometheusRefresh time.Duration

	o11yKey string
	o11yDS  string

	cooldown time.Duration
	startedC chan any

	errorC  chan kv.Error
	statusC chan []string

	logger *slog.Logger
}

// EntryPoint is the common starting point for test and production
func EntryPoint(ctx context.Context, opts *serverOpts) (errs []kv.Error) {

	if opts.cooldown == 0 {
		opts.cooldown = time.Duration(2 * time.Second)
	}

	if len(opts.ipPort) == 0 {
		opts.ipPort = "0.0.0.0:8080"
	}

	if len(opts.certPemFn) == 0 {
		opts.certPemFn = "testing.crt"
	}
	if len(opts.certKeyFn) == 0 {
		opts.certKeyFn = "testing.key"
	}

	opts.logger.Info("starting", "revision", runtime.BuildInfo.ShortRevision, "go", runtime.BuildInfo.GoVersion, "platform", runtime.BuildInfo.OS+"/"+runtime.BuildInfo.Arch)

	// Start supervisor channel for the main server goroutine
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		// Absorb any server issues occuring during shutdown
		defer func() {
			_ = recover()
		}()
		cancel()
	}()

	opts.errorC, opts.statusC = processMonitor(ctx, cancel, opts)

	// Monitoring of cluster state etc
	clusterMonitor := make(chan struct{}, 1)
	refreshState := time.Duration(15 * time.Second)
	go server.InitiateK8s(ctx, opts.cfgNamespace, opts.cfgConfigMap, clusterMonitor, refreshState, *opts.logger, opts.errorC)
	select {
	case <-clusterMonitor:
		opts.logger.Debug("kubernetes monitoring active")
	case <-time.After(2 * time.Second):
		opts.logger.Warn("kubernetes monitoring not available")
	case <-ctx.Done():
		return
	}

	if err := startServices(ctx, opts, opts.statusC, opts.errorC); err != nil {
		return []kv.Error{err}
	}

	opts.logger.Debug("server initiation complete")
	func(startedC chan any) {
		defer func() {
			_ = recover()
		}()
		close(startedC)
	}(opts.startedC)

	<-ctx.Done()

	return nil
}

// processMonitor is used to listen for process termination signals and errors
func processMonitor(ctx context.Context, cancel context.CancelFunc, opts *serverOpts) (errorC chan kv.Error, statusC chan []string) {

	// Reporting channels for the main server goroutine
	errorC = make(chan kv.Error, 1)
	statusC = make(chan []string, 1)

	// Setup a channel for graceful shutdown on a CTRL-C etc
	killC := make(chan os.Signal, 1)

	// Asynchronous listener for SIGINT, SIGTERM, and SIGHUP ... signals
	go func() {
		for {
			select {
			case statusMsgs := <-statusC:
				// Output any errors received asynchronously
				if len(statusMsgs) == 1 {
					opts.logger.Info(statusMsgs[0])
				} else {
					if len(statusMsgs) != 0 {
						opts.logger.Info(statusMsgs[0], statusMsgs[1:])
					}
				}
			case err := <-errorC:
				// Catch a nil arising from the channel being closed
				if err != nil {
					opts.logger.Warn(err.Error())
				}
				opts.logger.Info("server shutdown due to fatal error")

				defer func() {
					_ = recover()
				}()
				if killC != nil {
					close(killC)
				}
				return
			case <-ctx.Done():
				defer func() {
					_ = recover()
				}()
				if killC != nil {
					close(killC)
				}
				return
			case <-killC:
				defer func() {
					_ = recover()
				}()
				cancel()
				return
			}
		}
	}()

	// Add the SIGINT, SIGTERM, and SIGHUP signals to the channel listener
	signal.Notify(killC, os.Interrupt, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	return errorC, statusC
}

// startServices starts any asynchronously processing dependencies for this server
func startServices(ctx context.Context, opts *serverOpts, statusC chan []string, errorC chan kv.Error) (err kv.Error) {

	if err = server.StartPrometheusExporter(ctx, opts.prometheusAddr, &server.Resources{}, opts.prometheusRefresh, *opts.logger); err != nil {
		return err.With("stack", stack.Trace().TrimRuntime())
	}

	var (
		span   trace.Span
		tracer trace.Tracer
	)

	if len(opts.o11yKey) != 0 && len(opts.o11yDS) != 0 {
		bag := baggage.FromContext(ctx)

		otelOpts := server.StartTelemetryOpts{
			NodeName:    opts.cfgHost,
			ServiceName: opts.serviceID,
			ProjectID:   runtime.BuildInfo.ProjectPath,
			ApiKey:      opts.o11yKey,
			Dataset:     opts.o11yDS,
			ApiEndpoint: opts.cfgHost,
			Cooldown:    opts.cooldown,
			Bag:         &bag,
		}

		baggageItems := [][]string{
			{"service.name", opts.serviceID},
			{"service.host", opts.cfgHost},
		}

		for _, baggageItem := range baggageItems {
			if len(baggageItem) != 2 {
				continue
			}
			member, errGo := baggage.NewMember(baggageItem[0], baggageItem[1])
			if errGo != nil {
				opts.logger.Warn(errGo.Error())
				continue
			}
			if bag, errGo = bag.SetMember(member); errGo != nil {
				opts.logger.Warn(errGo.Error())
			}
		}

		ctx, err = server.StartTelemetry(ctx, otelOpts, *opts.logger)
		if err != nil {
			opts.logger.Warn(err.Error())
		}

		ctx = baggage.ContextWithBaggage(ctx, bag)
	}

	// Create a server span to cover our dependencies, general processing and provisioned interfaces
	tracer = otel.GetTracerProvider().Tracer(runtime.BuildInfo.ProjectPath)
	span = trace.SpanFromContext(ctx)

	// Initialize a component monitor (poor mans supervisor) to allow health checking to be implemented
	// across all dependencies and internal components
	comps := components.InitComponentTracking(ctx)
	initHealthMonitoring(ctx, opts.serviceID, comps, opts.logger)

	// Start the main server goroutine
	if err = startServer(ctx, opts, comps); err != nil {
		return err
	}

	if span != nil {
		ctx, span := tracer.Start(ctx, "local dependencies")
		span.AddEvent("started")
		go func(span trace.Span) {
			<-ctx.Done()
			opts.logger.Debug("stopping")
			span.AddEvent("stopping")
			span.End()
		}(span)
	}

	return nil
}
