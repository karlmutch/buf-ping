package main

// This file contains the code used to initiate the server when not running under test.
// Any CLI procesisng will be performed by tis function then it will invoke the EntryPoint
// function to run the server proper.

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/karlmutch/go-service/pkg/process"
	"github.com/karlmutch/go-service/pkg/runtime"

	"github.com/shirou/gopsutil/v3/host"
)

// main is the standard entrypoint for when the test suite is not being run
func main() {

	serverID := "ping-server"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, errGo := host.HostID()
	if errGo != nil {
		fmt.Println("this OS lacks unique host identifiers", "error", errGo.Error())
		os.Exit(-1)
	}

	if runtime.BuildInfo.OS != "darwin" {
		// Check for single instances
		if _, err := process.NewExclusive(ctx, serverID); err != nil {
			fmt.Println(serverID+" already running", "error", err.Error())
			os.Exit(-2)
		}
	}

	opts := serverOpts{
		serviceID:         serverID,
		logger:            slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		prometheusRefresh: time.Duration(15 * time.Second),
		startedC:          make(chan any),
	}

	// func is used to allow for defer's and system wide shutdown when the EntryPoint function exits
	func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if initErrs := EntryPoint(ctx, &opts); len(initErrs) != 0 {
			errSeen := false
			for _, anErr := range initErrs {
				if anErr != nil {
					opts.logger.Error(anErr.Error())
					errSeen = true
				}
			}
			if errSeen {
				os.Exit(-3)
			}
		}
	}()

	cooldown := time.Duration(time.Second)
	opts.logger.Debug("shutting down", "cooldown", cooldown.String())

	// Wait for any internal go routines etc to shut down
	time.Sleep(cooldown)
}
