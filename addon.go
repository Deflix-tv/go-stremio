package stremio

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

// CatalogHandler is the callback for catalog requests for a specific type (like "movie").
// The id parameter is the catalog ID that you specified yourself in the CatalogItem objects in the Manifest.
type CatalogHandler func(id string) ([]MetaPreviewItem, error)

// StreamHandler is the callback for stream requests for a specific type (like "movie").
// The id parameter can be for example an IMDb ID if your addon handles the "movie" type.
type StreamHandler func(id string) ([]StreamItem, error)

// Options are the options that can be used to configure the addon.
type Options struct {
	// The interface to bind to.
	// "0.0.0.0" to bind to all interfaces. "localhost" to *exclude* requests from other machines.
	// Default "localhost".
	BindAddr string
	// The port to listen on.
	// Default 8080.
	Port int
	// The log level.
	// Only logs with the same or a higher log level will be shown.
	// For example when you set it to "info", info, warn, error, fatal and panic logs will be shown, but no debug or trace logs.
	// Must be parseable by logrus: https://pkg.go.dev/github.com/sirupsen/logrus?tab=doc#ParseLevel
	// Default "info".
	LogLevel string
	// URL to redirect to when someone requests the root of the handler instead of the manifest, catalog, stream etc.
	// When no value is set, it will lead to a "404 Not Found" response.
	// Default "".
	RedirectURL string
}

// DefaultOptions is an Options object with default values.
var DefaultOptions = Options{
	BindAddr:    "localhost",
	Port:        8080,
	LogLevel:    "info",
	RedirectURL: "",
}

// Addon represents a remote addon.
// You can create one with NewAddon() and then run it with Run().
type Addon struct {
	manifest        Manifest
	catalogHandlers map[string]CatalogHandler
	streamHandlers  map[string]StreamHandler
	opts            Options
}

// NewAddon creates a new Addon object that can be started with Run().
// A proper manifest must be supplied, but all but one handler can be nil in case you only want to handle specific requests and opts can be the zero value of Options.
func NewAddon(manifest Manifest, catalogHandlers map[string]CatalogHandler, streamHandlers map[string]StreamHandler, opts Options) (Addon, error) {
	// Precondition checks
	if manifest.ID == "" || manifest.Name == "" || manifest.Description == "" || manifest.Version == "" {
		return Addon{}, errors.New("An empty manifest was passed")
	} else if catalogHandlers == nil && streamHandlers == nil {
		return Addon{}, errors.New("No handler was passed")
	}
	// Set default values
	if opts.BindAddr == "" {
		opts.BindAddr = DefaultOptions.BindAddr
	}
	if opts.LogLevel == "" {
		opts.LogLevel = DefaultOptions.LogLevel
	}
	if opts.Port == 0 {
		opts.Port = DefaultOptions.Port
	}

	return Addon{
		manifest:        manifest,
		catalogHandlers: catalogHandlers,
		streamHandlers:  streamHandlers,
		opts:            opts,
	}, nil
}

// Run starts the remote addon. It sets up an HTTP server that handles requests to "/manifest.json" etc. and gracefully handles shutdowns.
func (a Addon) Run() {
	setLogLevel(a.opts.LogLevel)

	log.Info("Setting up server...")
	r := mux.NewRouter()
	s := r.Methods("GET").Subrouter()
	s.Use(timerMiddleware,
		corsMiddleware, // Stremio doesn't show stream responses when no CORS middleware is used!
		handlers.ProxyHeaders,
		recoveryMiddleware,
		loggingMiddleware)
	s.HandleFunc("/health", healthHandler)

	// Stremio endpoints

	s.HandleFunc("/manifest.json", createManifestHandler(a.manifest))
	if a.catalogHandlers != nil {
		s.HandleFunc("/catalog/{type}/{id}.json", createCatalogHandler(a.catalogHandlers))
	}
	if a.streamHandlers != nil {
		s.HandleFunc("/stream/{type}/{id}.json", createStreamHandler(a.streamHandlers))
	}

	// Additional endpoints

	// Root redirects to website
	if a.opts.RedirectURL != "" {
		s.HandleFunc("/", createRootHandler(a.opts.RedirectURL))
	}

	addr := a.opts.BindAddr + ":" + strconv.Itoa(a.opts.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: s,
		// Timeouts to avoid Slowloris attacks
		ReadTimeout:    time.Second * 5,
		WriteTimeout:   time.Second * 15,
		IdleTimeout:    time.Second * 60,
		MaxHeaderBytes: 1 * 1000, // 1 KB
	}

	log.Info("Finished setting up server")

	stopping := false
	stoppingPtr := &stopping

	log.WithField("address", addr).Info("Starting server")
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			if !*stoppingPtr {
				log.WithError(err).Fatal("Couldn't start server")
			} else {
				log.WithError(err).Fatal("Error in srv.ListenAndServe() during server shutdown (probably context deadline expired before the server could shutdown cleanly)")
			}
		}
	}()

	// Timed logger for easier debugging with logs
	go func() {
		for {
			log.Trace("...")
			time.Sleep(time.Second)
		}
	}()

	// Graceful shutdown

	c := make(chan os.Signal, 1)
	// Accept SIGINT (Ctrl+C) and SIGTERM (`docker stop`)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	sig := <-c
	log.WithField("signal", sig).Info("Received signal, shutting down server...")
	*stoppingPtr = true
	// Create a deadline to wait for. `docker stop` gives us 10 seconds.
	// No need to get the cancel func and defer calling it, because srv.Shutdown() will consider the timeout from the context.
	ctx, _ := context.WithTimeout(context.Background(), 9*time.Second)
	// Doesn't block if no connections, but will otherwise wait until the timeout deadline
	if err := srv.Shutdown(ctx); err != nil {
		log.WithError(err).Fatal("Error shutting down server")
	}
	log.Info("Finished shutting down server")
}

func setLogLevel(logLevel string) {
	logrusLevel, err := log.ParseLevel(logLevel)
	if err != nil {
		log.WithField("logLevel", logLevel).Fatal("Unknown logLevel")
	}
	log.SetLevel(logrusLevel)
}
