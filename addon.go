package stremio

import (
	"errors"
	"flag"
	"fmt"
	netpprof "net/http/pprof"
	"os"
	"os/signal"
	"reflect"
	"runtime/pprof"
	"strconv"
	"syscall"
	"time"

	"github.com/gofiber/adaptor"
	"github.com/gofiber/fiber"
	"github.com/gofiber/fiber/middleware"
	"go.uber.org/zap"
)

// ManifestCallback is the callback for manifest requests, so mostly addon installations.
// You can use the callback to *prevent* users from installing your addon.
// The userData parameter depends on whether you called `RegisterUserData()` before:
// If not, a simple string will be passed. It's empty if the user didn't provide user data.
// If yes, a pointer to an object you registered will be passed. It's nil if the user didn't provide user data.
// Return an HTTP status code >= 400 to stop further processing and let the addon return that exact status code.
// Any status code < 400 will lead to the manifest being returned with a 200 OK status code in the response.
type ManifestCallback func(userData interface{}) int

// CatalogHandler is the callback for catalog requests for a specific type (like "movie").
// The id parameter is the catalog ID that you specified yourself in the CatalogItem objects in the Manifest.
// The userData parameter depends on whether you called `RegisterUserData()` before:
// If not, a simple string will be passed. It's empty if the user didn't provide user data.
// If yes, a pointer to an object you registered will be passed. It's nil if the user didn't provide user data.
type CatalogHandler func(id string, userData interface{}) ([]MetaPreviewItem, error)

// StreamHandler is the callback for stream requests for a specific type (like "movie").
// The id parameter can be for example an IMDb ID if your addon handles the "movie" type.
// The userData parameter depends on whether you called `RegisterUserData()` before:
// If not, a simple string will be passed. It's empty if the user didn't provide user data.
// If yes, a pointer to an object you registered will be passed. It's nil if the user didn't provide user data.
type StreamHandler func(id string, userData interface{}) ([]StreamItem, error)

// Addon represents a remote addon.
// You can create one with NewAddon() and then run it with Run().
type Addon struct {
	manifest          Manifest
	catalogHandlers   map[string]CatalogHandler
	streamHandlers    map[string]StreamHandler
	opts              Options
	logger            *zap.Logger
	customMiddlewares []customMiddleware
	customEndpoints   []customEndpoint
	manifestCallback  ManifestCallback
	userDataType      reflect.Type
}

func init() {
	// We need to overwrite the usage of the default FlagSet to hide the flags defined by Fiber
	flag.CommandLine.Usage = usage
}

// NewAddon creates a new Addon object that can be started with Run().
// A proper manifest must be supplied, but manifestCallback and all but one handler can be nil in case you only want to handle specific requests and opts can be the zero value of Options.
func NewAddon(manifest Manifest, catalogHandlers map[string]CatalogHandler, streamHandlers map[string]StreamHandler, opts Options) (*Addon, error) {
	// Precondition checks
	if manifest.ID == "" || manifest.Name == "" || manifest.Description == "" || manifest.Version == "" {
		return nil, errors.New("An empty manifest was passed")
	} else if catalogHandlers == nil && streamHandlers == nil {
		return nil, errors.New("No handler was passed")
	} else if (opts.CachePublicCatalogs && opts.CacheAgeCatalogs == 0) ||
		(opts.CachePublicStreams && opts.CacheAgeStreams == 0) {
		return nil, errors.New("Enabling public caching only makes sense when also setting a cache age")
	} else if (opts.HandleEtagCatalogs && opts.CacheAgeCatalogs == 0) ||
		(opts.HandleEtagStreams && opts.CacheAgeStreams == 0) {
		return nil, errors.New("ETag handling only makes sense when also setting a cache age")
	} else if opts.DisableRequestLogging && (opts.LogIPs || opts.LogUserAgent) {
		return nil, errors.New("Enabling IP or user agent logging doesn't make sense when disabling request logging")
	} else if opts.Logger != nil && opts.LoggingLevel != "" {
		return nil, errors.New("Setting a logging level in the options doesn't make sense when you already set a custom logger")
	}

	// Set default values
	if opts.BindAddr == "" {
		opts.BindAddr = DefaultOptions.BindAddr
	}
	if opts.LoggingLevel == "" {
		opts.LoggingLevel = DefaultOptions.LoggingLevel
	}
	if opts.Port == 0 {
		opts.Port = DefaultOptions.Port
	}

	// Configure logger if no custom one is set
	if opts.Logger == nil {
		var err error
		if opts.Logger, err = NewLogger(opts.LoggingLevel); err != nil {
			return nil, fmt.Errorf("Couldn't create new logger: %w", err)
		}
	}

	// Create and return addon
	return &Addon{
		manifest:        manifest,
		catalogHandlers: catalogHandlers,
		streamHandlers:  streamHandlers,
		opts:            opts,
		logger:          opts.Logger,
	}, nil
}

// RegisterUserData registers the type of userData, so the addon can automatically unmarshal user data into an object of this type
// and pass the object into the manifest callback or catalog and stream handlers.
func (a *Addon) RegisterUserData(userDataObject interface{}) {
	t := reflect.TypeOf(userDataObject)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	a.userDataType = t
}

// DecodeUserData decodes the request's user data and returns the result.
// It's useful when you add custom endpoints to the addon that don't have a userData parameter
// like the ManifestCallback, CatalogHandler and StreamHandler have.
// The param value must match the URL parameter you used when creating the custom endpoint,
// for example when using `AddEndpoint("GET", "/:userData/ping", customEndpoint)` you must pass "userData".
func (a *Addon) DecodeUserData(param string, c *fiber.Ctx) (interface{}, error) {
	data := c.Params(param, "")
	return decodeUserData(data, a.userDataType, a.logger, a.opts.UserDataIsBase64)
}

// AddMiddleware appends a custom middleware to the chain of existing middlewares.
// Set path to an empty string or "/" to let the middleware apply to all routes.
// Don't forget to call c.Next() on the Fiber context!
func (a *Addon) AddMiddleware(path string, mw func(*fiber.Ctx)) {
	customMW := customMiddleware{
		path: path,
		mw:   mw,
	}
	a.customMiddlewares = append(a.customMiddlewares, customMW)
}

// AddEndpoint adds a custom endpoint (a route and its handler).
// If you want to be able to access custom user data, you can use a path like this:
// "/:userData/foo" and then either deal with the data yourself
// by using `c.Params("userData", "")` in the handler,
// or use the convenience method `DecodeUserData("userData", c)`.
func (a *Addon) AddEndpoint(method, path string, handler func(*fiber.Ctx)) {
	customEndpoint := customEndpoint{
		method:  method,
		path:    path,
		handler: handler,
	}
	a.customEndpoints = append(a.customEndpoints, customEndpoint)
}

// SetManifestCallback sets the manifest callback
func (a *Addon) SetManifestCallback(callback ManifestCallback) {
	a.manifestCallback = callback
}

// Run starts the remote addon. It sets up an HTTP server that handles requests to "/manifest.json" etc. and gracefully handles shutdowns.
func (a *Addon) Run() {
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
		app.Use(createLoggingMiddleware(logger, a.opts.LogIPs, a.opts.LogUserAgent))
	}
	app.Use(corsMiddleware()) // Stremio doesn't show stream responses when no CORS middleware is used!
	for _, customMW := range a.customMiddlewares {
		app.Use(customMW.path, customMW.mw)
	}
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

	// In Fiber optional parameters don't work at the beginning of the URL, so we have to register two routes each
	manifestHandler := createManifestHandler(a.manifest, logger, a.manifestCallback, a.userDataType, a.opts.UserDataIsBase64)
	app.Get("/manifest.json", manifestHandler)
	app.Get("/:userData/manifest.json", manifestHandler)
	if a.catalogHandlers != nil {
		catalogHandler := createCatalogHandler(a.catalogHandlers, a.opts.CacheAgeCatalogs, a.opts.CachePublicCatalogs, a.opts.HandleEtagCatalogs, logger, a.userDataType, a.opts.UserDataIsBase64)
		app.Get("/catalog/:type/:id.json", catalogHandler)
		app.Get("/:userData/catalog/:type/:id.json", catalogHandler)
	}
	if a.streamHandlers != nil {
		streamHandler := createStreamHandler(a.streamHandlers, a.opts.CacheAgeStreams, a.opts.CachePublicStreams, a.opts.HandleEtagStreams, logger, a.userDataType, a.opts.UserDataIsBase64)
		app.Get("/stream/:type/:id.json", streamHandler)
		app.Get("/:userData/stream/:type/:id.json", streamHandler)
	}

	// Additional endpoints

	// Root redirects to website
	if a.opts.RedirectURL != "" {
		app.Get("/", createRootHandler(a.opts.RedirectURL, logger))
	}

	// Custom endpoints
	for _, customEndpoint := range a.customEndpoints {
		app.Add(customEndpoint.method, customEndpoint.path, customEndpoint.handler)
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
