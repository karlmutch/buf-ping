package main

// This file contain contains the implementation of the health checking
// features of this server.  It combiones the component checking of the
// go-service library with that of the connect-go health reporting library

import (
	"context"
	"log/slog"

	"connectrpc.com/grpchealth"

	"github.com/karlmutch/go-service/pkg/components"
)

var (
	serverHealth = grpchealth.NewStaticChecker()
)

type Checker struct{}

func (c *Checker) Check(ctx context.Context, request *grpchealth.CheckRequest) (response *grpchealth.CheckResponse, err error) {

	// If there is no service specified then be conservatibe and mark everything as down
	if request.Service == "" {
		return &grpchealth.CheckResponse{Status: grpchealth.StatusNotServing}, nil
	}

	return serverHealth.Check(ctx, request)
}

func AddStaticChecker(ctx context.Context, service string) (err error) {
	serverHealth.SetStatus(service, grpchealth.StatusNotServing)
	return nil
}

func initHealthMonitoring(ctx context.Context, serviceID string, comps *components.Components, logger *slog.Logger) {

	serverHealth.SetStatus(serviceID, grpchealth.StatusNotServing)

	listenerC := make(chan bool)

	comps.AddListener(listenerC)

	go func() {

		// Leave in its own defer in the event it causes a panic
		defer close(listenerC)

		defer func() {
			serverHealth.SetStatus(serviceID, grpchealth.StatusNotServing)
		}()

		healthy := false  // assume the server dependencies are down at the moment
		oldHealth := true // setting the old state to up will cause the initiate state to be printed

		for {
			select {
			case up := <-listenerC:
				healthy = up
				if up {
					serverHealth.SetStatus(serviceID, grpchealth.StatusServing)
				} else {
					serverHealth.SetStatus(serviceID, grpchealth.StatusNotServing)
				}
				if oldHealth != healthy {
					state := "healthy"
					if !healthy {
						state = "unhealthy"
					}
					oldHealth = healthy

					logger.Warn("health status transitioned", "state", state)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}
