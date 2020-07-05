package stremio

import (
	"errors"
	"flag"
	"fmt"
	netpprof "net/http/pprof"
	"os"
	"os/signal"
	"runtime/pprof"
	"strconv"
	"syscall"
	"time"

	"github.com/gofiber/adaptor"
	"github.com/gofiber/fiber"
	"github.com/gofiber/fiber/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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
	// For example when you set it to "info", info, warn and error logs will be shown, but no debug logs.
	// Accepts "debug", "info", "warn" and "error".
	// Default "info".
	LogLevel string
	// Flag for indicating whether requests should be logged.
	// Default false (meaning requests will be logged by default).
	DisableRequestLogging bool
	// Flag for indicating whether IP addresses should be logged.
	// Default false (meaning IP addresses will be logged by default).
	DisableIPlogging bool
	// Flag for indicating whether the user agent header should be logged.
	// Default false (meaning the user agent header will be logged by default).
	DisableUserAgentLogging bool
	// URL to redirect to when someone requests the root of the handler instead of the manifest, catalog, stream etc.
	// When no value is set, it will lead to a "404 Not Found" response.
	// Default "".
	RedirectURL string
	// Flag for indicating whether you want to expose URL handlers for the Go profiler.
	// The URLs are be the standard ones: "/debug/pprof/...".
	// Default false.
	Profiling bool
	// Duration of client/proxy-side cache for responses from the catalog endpoint.
	// Helps reducing number of requsts and transferred data volume to/from the server.
	// The result is not cached by the SDK on the server side, so if two *separate* users make a reqeust,
	// and no proxy cached the response, your CatalogHandler will be called twice.
	// Default 0.
	CacheAgeCatalogs time.Duration
	// Same as CacheAgeCatalogs, but for streams.
	CacheAgeStreams time.Duration
	// Flag for indicating to proxies whether they are allowed to cache responses from the catalog endpoint.
	// Default false.
	CachePublicCatalogs bool
	// Same as CachePublicCatalogs, but for streams.
	CachePublicStreams bool
	// Flag for indicating whether the "ETag" header should be set and the "If-None-Match" header checked.
	// Helps reducing the transferred data volume from the server even further.
	// Only makes sense when setting a non-zero CacheAgeCatalogs.
	// Leads to a slight computational overhead due to every CatalogHandler result being hashed.
	// Default false.
	HandleEtagCatalogs bool
	// Same as HandleEtagCatalogs, but for streams.
	HandleEtagStreams bool
}

// DefaultOptions is an Options object with default values.
// For fields that aren't set here the zero value is the default value.
var DefaultOptions = Options{
	BindAddr: "localhost",
	Port:     8080,
	LogLevel: "info",
}

// Addon represents a remote addon.
// You can create one with NewAddon() and then run it with Run().
type Addon struct {
	manifest        Manifest
	catalogHandlers map[string]CatalogHandler
	streamHandlers  map[string]StreamHandler
	opts            Options
	logger          *zap.Logger
}

func init() {
	// We need to overwrite the usage of the default FlagSet to hide the flags defined by Fiber
	flag.CommandLine.Usage = usage
}

// NewAddon creates a new Addon object that can be started with Run().
// A proper manifest must be supplied, but all but one handler can be nil in case you only want to handle specific requests and opts can be the zero value of Options.
func NewAddon(manifest Manifest, catalogHandlers map[string]CatalogHandler, streamHandlers map[string]StreamHandler, opts Options) (Addon, error) {
	// Precondition checks
	if manifest.ID == "" || manifest.Name == "" || manifest.Description == "" || manifest.Version == "" {
		return Addon{}, errors.New("An empty manifest was passed")
	} else if catalogHandlers == nil && streamHandlers == nil {
		return Addon{}, errors.New("No handler was passed")
	} else if (opts.HandleEtagCatalogs && !opts.CachePublicCatalogs) ||
		(opts.HandleEtagStreams && !opts.CachePublicStreams) {
		return Addon{}, errors.New("ETags only make sense when also setting a cache age")
	} else if opts.DisableRequestLogging && (opts.DisableIPlogging || opts.DisableUserAgentLogging) {
		return Addon{}, errors.New("Enabling IP or user agent logging doesn't make sense when disabling request logging")
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

	// Configure logger
	logLevel, err := parseZapLevel(opts.LogLevel)
	if err != nil {
		return Addon{}, fmt.Errorf("Couldn't parse log level: %w", err)
	}
	logConfig := zap.NewDevelopmentConfig()
	logConfig.Level = zap.NewAtomicLevelAt(logLevel)
	// Deactivate stacktraces for warn level.
	logConfig.Development = false
	// Mix between zap's development and production EncoderConfig and other changes.
	logConfig.EncoderConfig = zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.RFC3339TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   nil,
	}
	logger, err := logConfig.Build()
	if err != nil {
		return Addon{}, fmt.Errorf("Couldn't create logger: %w", err)
	}

	return Addon{
		manifest:        manifest,
		catalogHandlers: catalogHandlers,
		streamHandlers:  streamHandlers,
		opts:            opts,
		logger:          logger,
	}, nil
}

// Run starts the remote addon. It sets up an HTTP server that handles requests to "/manifest.json" etc. and gracefully handles shutdowns.
func (a Addon) Run() {
	logger := a.logger
	defer logger.Sync()

	logger.Info("Setting up server...")
	app := fiber.New(&fiber.Settings{
		ErrorHandler: func(ctx *fiber.Ctx, err error) {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
				logger.Error("Fiber's error handler was called", zap.Error(e))
			} else {
				logger.Error("Fiber's error handler was called", zap.Error(err))
			}
			ctx.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
			ctx.Status(code).SendString("An internal server error occurred")
		},
		BodyLimit:             0,
		DisableStartupMessage: true,
		ReadTimeout:           5 * time.Second,
		WriteTimeout:          15 * time.Second,
		IdleTimeout:           60 * time.Second, // 1m
		ReadBufferSize:        1000,             // 1 KB
	})
	app.Use(middleware.Recover())
	if !a.opts.DisableRequestLogging {
		app.Use(createLoggingMiddleware(logger, !a.opts.DisableIPlogging, !a.opts.DisableUserAgentLogging))
	}
	app.Use(corsMiddleware()) // Stremio doesn't show stream responses when no CORS middleware is used!
	app.Get("/health", createHealthHandler(logger))
	// Optional profiling
	if a.opts.Profiling {
		group := app.Group("/debug/pprof")

		group.Get("/", func(c *fiber.Ctx) {
			c.Set(fiber.HeaderContentType, fiber.MIMETextHTML)
			adaptor.HTTPHandlerFunc(netpprof.Index)(c)
		})
		for _, p := range pprof.Profiles() {
			group.Get("/"+p.Name(), adaptor.HTTPHandler(netpprof.Handler(p.Name())))
		}
		group.Get("/cmdline", adaptor.HTTPHandlerFunc(netpprof.Cmdline))
		group.Get("/profile", adaptor.HTTPHandlerFunc(netpprof.Profile))
		group.Get("/trace", adaptor.HTTPHandlerFunc(netpprof.Trace))
	}

	// Stremio endpoints

	app.Get("/manifest.json", createManifestHandler(a.manifest, logger))
	if a.catalogHandlers != nil {
		app.Get("/catalog/:type/:id.json", createCatalogHandler(a.catalogHandlers, a.opts.CacheAgeCatalogs, a.opts.CachePublicCatalogs, a.opts.HandleEtagCatalogs, logger))
	}
	if a.streamHandlers != nil {
		app.Get("/stream/:type/:id.json", createStreamHandler(a.streamHandlers, a.opts.CacheAgeStreams, a.opts.CachePublicStreams, a.opts.HandleEtagStreams, logger))
	}

	// Additional endpoints

	// Root redirects to website
	if a.opts.RedirectURL != "" {
		app.Get("/", createRootHandler(a.opts.RedirectURL, logger))
	}

	logger.Info("Finished setting up server")

	stopping := false
	stoppingPtr := &stopping

	addr := a.opts.BindAddr + ":" + strconv.Itoa(a.opts.Port)
	logger.Info("Starting server", zap.String("address", addr))
	go func() {
		if err := app.Listen(addr); err != nil {
			if !*stoppingPtr {
				logger.Fatal("Couldn't start server", zap.Error(err))
			} else {
				logger.Fatal("Error in srv.ListenAndServe() during server shutdown (probably context deadline expired before the server could shutdown cleanly)", zap.Error(err))
			}
		}
	}()

	// Graceful shutdown

	c := make(chan os.Signal, 1)
	// Accept SIGINT (Ctrl+C) and SIGTERM (`docker stop`)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	sig := <-c
	logger.Info("Received signal, shutting down server...", zap.Stringer("signal", sig))
	*stoppingPtr = true
	// Graceful shutdown, waiting for all current requests to finish without accepting new ones.
	if err := app.Shutdown(); err != nil {
		logger.Fatal("Error shutting down server", zap.Error(err))
	}
	logger.Info("Finished shutting down server")
}

// Logger returns the addon's logger.
// It's recommended to use this logger for logging in addons
// so that the logging output is consistent.
// You can also change its configuration this way,
// as it's a pointer to the logger that's used by the SDK.
func (a Addon) Logger() *zap.Logger {
	return a.logger
}

func parseZapLevel(logLevel string) (zapcore.Level, error) {
	switch logLevel {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	}
	return 0, errors.New(`unknown log level - only knows ["debug", "info", "warn", "error"]`)
}
