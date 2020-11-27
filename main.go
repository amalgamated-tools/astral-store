package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	stdlog "log"

	"github.com/amalgamated-tools/astral-store/config"
	"github.com/amalgamated-tools/astral-store/web"
	"github.com/getsentry/sentry-go"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-service/debug"
)

// GitCommit is used as the application version string, set by LD flags.
var GitCommit string

// log is a structured hclog logger used as the default package logger.
var log hclog.InterceptLogger

func main() {
	os.Exit(realMain(os.Args, os.Stdout, os.Stderr))
}

func setupSentry(cfg *config.Config) {
	// Add gitcommit to release
	opts := sentry.ClientOptions{
		Debug:       true,
		DebugWriter: os.Stderr,
		Release:     GitCommit,
		Environment: os.Getenv("SENTRY_ENV"),
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			log.Debug(
				"Sending event to Sentry",
				"eventID",
				event.EventID,
				"message",
				event.Message,
				"exceptionValue",
				event.Exception[0].Value,
			)
			return event
		},
	}
	// this sets up the global sentry
	sentryErr := sentry.Init(opts)
	if sentryErr != nil {
		panic(sentryErr)
	}

	defer sentry.Flush(2 * time.Second)
}

// setupLogging allows for reconfiguration of the logger.
func setupLogging(cfg *config.Config) {
	log = hclog.NewInterceptLogger(&hclog.LoggerOptions{
		Name:       "astral-store",
		Level:      hclog.LevelFromString(string(cfg.Server.LogLevel)),
		JSONFormat: cfg.Server.LogFormat == config.LogFormatJSON,
	})
	hclog.SetDefault(log)

	// Plumb up the standard logger to the hclog logger, using InferLevels to
	// map (eg) '[DEBUG] foo' to the debug level. The standard logger should
	// not be used, but this makes sure that the logs of any dependencies that
	// use the standard logger directly are also handled by the hclog logger.
	opts := &hclog.StandardLoggerOptions{InferLevels: true}
	stdlog.SetOutput(log.Named("stdlog").StandardWriter(opts))
	stdlog.SetPrefix("")
	stdlog.SetFlags(0)
}

func realMain(args []string, stdout, stderr io.Writer) int {
	// return handleSignals(web)
	setupLogging(config.Default())
	// Ensure we have a config path. (go run main.go local.json or tf-vcs local.json)
	if len(args) != 2 {
		fmt.Fprintln(stderr, usage())
		return 1
	}

	// Parse the config file.
	cfg, err := config.Parse(args[1])
	if err != nil {
		log.Error("Failed to parse config", "error", err)
		sentry.CaptureException(err)
		return 1
	}

	// Reconfigure the logger using the given config.
	setupLogging(cfg)
	log.Debug("Configuration", "config", cfg)

	setupSentry(cfg)

	webApp, err := web.New(cfg, log)
	if err != nil {
		log.Error("Failed creating web", "error", err)
		sentry.CaptureException(err)
		return 1
	}
	// Start the vcwebs service.
	if err := webApp.Start(); err != nil {
		log.Error("Failed to start the web service", "error", err)
		sentry.CaptureException(err)
		return 1
	}

	return handleSignals(webApp)

}

func handleSignals(web *web.Web) int {
	// Dump stack on SIGUSR1.
	debug.SignalStackDump(syscall.SIGUSR1)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	s := <-signalCh
	log.Info("Received signal, shutting down", "signal", s)

	// Begin graceful shutdown.
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		err := web.Shutdown()
		if err != nil {
			log.Error("svc.Shutdown returned errors", err)
		}
	}()

	select {
	case <-doneCh:
		log.Info("Graceful shutdown complete")
		return 0
	case s := <-signalCh:
		log.Error("Graceful shutdown aborted!", "signal", s)
		return 1
	}
}

// usage returns the CLI usage string.
func usage() string {
	return strings.TrimSpace(`
usage: astral-store <config file>
`)
}
