package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpchealth"
	"connectrpc.com/grpcreflect"
	"connectrpc.com/otelconnect"

	"github.com/go-stack/stack"
	"github.com/rs/cors"

	"buf.build/gen/go/karlmutch/buf-ping/connectrpc/go/ping/v1/pingv1connect"

	"github.com/karlmutch/buf-ping/pkg/ping"
	"github.com/karlmutch/go-service/pkg/components"

	"github.com/karlmutch/kv"
)

// newCORS will setup the cors package for the server
func newCORS() (coresProfile *cors.Cors) {
	coresProfile = cors.New(cors.Options{
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
		},
		AllowOriginFunc: func(origin string) bool {
			// Allow all origins, a little too permissive for now
			return true
		},
		AllowedHeaders: []string{"*"},
		ExposedHeaders: []string{
			"Accept",
			"Accept-Encoding",
			"Accept-Post",
			"Connect-Accept-Encoding",
			"Connect-Content-Encoding",
			"Content-Encoding",
			"Grpc-Accept-Encoding",
			"Grpc-Encoding",
			"Grpc-Message",
			"Grpc-Status",
			"Grpc-Status-Details-Bin",
			"X-Grpc-Test-Echo-Initial",
			"X-Grpc-Test-Echo-Trailing-Bin",
		},
	})
	return coresProfile
}

func newTLSConfig() (tlsConfig *tls.Config) {

	// For more information about the `Modern Compatability` being used
	// please read https://wiki.mozilla.org/Security/Server_Side_TLS#Modern_compatibility.
	// This intentionally blocks compatability with TLS 1.2
	tlsConfig = &tls.Config{
		MinVersion:               tls.VersionTLS13,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
	}
	return tlsConfig
}

func startServer(ctx context.Context, opts *serverOpts, comps *components.Components) (err kv.Error) {

	pingServer := ping.NewPingServer(*opts.logger)

	compress1KB := connect.WithCompressMinBytes(1024)

	// otelconnect.NewInterceptor provides an interceptor that adds tracing and
	// metrics to both clients and handlers. By default, it uses OpenTelemetry's
	// global TracerProvider and MeterProvider, which you can configure by
	// following the OpenTelemetry documentation.
	interceptors := connect.WithInterceptors(otelconnect.NewInterceptor(otelconnect.WithTrustRemote()))

	// Combine everything into a single handler for nthe ping service route
	mux := http.NewServeMux()
	mux.Handle(pingv1connect.NewPingServiceHandler(pingServer, interceptors, compress1KB))

	// For more information please see, https://github.com/bufbuild/connect-grpchealth-go
	// The health checker is not given authentication checking
	mux.Handle(grpchealth.NewHandler(serverHealth, compress1KB))

	// Function that is used to add a grpc static checker to the connect grpchealth instance
	AddStaticChecker(ctx, pingv1connect.PingServiceName)

	// Reflection will use authentication
	mux.Handle(grpcreflect.NewHandlerV1(
		grpcreflect.NewStaticReflector(pingv1connect.PingServiceName),
		compress1KB,
		interceptors,
	))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(
		grpcreflect.NewStaticReflector(pingv1connect.PingServiceName),
		compress1KB,
		interceptors,
	))

	srvr := &http.Server{
		Addr:              opts.ipPort,
		ReadHeaderTimeout: time.Second,
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
		MaxHeaderBytes:    8 * 1024, // 8KiB
		TLSConfig:         newTLSConfig(),
		Handler:           newCORS().Handler(mux),
	}
	opts.logger.Info("TLS listener starting", "address", opts.ipPort)
	errGo := srvr.ListenAndServeTLS(opts.certPemFn, opts.certKeyFn)
	if errGo != nil {
		opts.errorC <- kv.Wrap(errGo).With("stack", stack.Trace().TrimRuntime())
	}

	comps.SetModule(opts.serviceID, false)
	func() {
		defer recover()
		close(opts.errorC)
	}()

	return nil
}
