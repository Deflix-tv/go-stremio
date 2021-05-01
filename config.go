package stremio

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Options are the options that can be used to configure the addon.
type Options struct {
	// The interface to bind to.
	// "0.0.0.0" to bind to all interfaces. "localhost" to *exclude* requests from other machines.
	// Default "localhost".
	BindAddr string
	// The port to listen on.
	// Default 8080.
	Port int
	// You can set a custom logger, or leave this empty to create a new one
	// with sane defaults and the LoggingLevel in these options.
	// If you already called `NewLogger()`, you should set that logger here.
	// Default nil.
	Logger *zap.Logger
	// The logging level.
	// Only logs with the same or a higher log level will be shown.
	// For example when you set it to "info", info, warn and error logs will be shown, but no debug logs.
	// Accepts "debug", "info", "warn" and "error".
	// Only required when not already setting the Logger in the options.
	// Default "info".
	LoggingLevel string
	// Configures zap's log encoding.
	// "console" will format a log line console-friendly.
	// "json" is better suited when using a centralized log solution like ELK, Graylog or Loki.
	// Default "console".
	LogEncoding string
	// Flag for indicating whether requests should be logged.
	// Default false (meaning requests will be logged by default).
	DisableRequestLogging bool
	// Flag for indicating whether IP addresses should be logged.
	// Default false.
	LogIPs bool
	// Flag for indicating whether the user agent header should be logged.
	// Default false.
	LogUserAgent bool
	// URL to redirect to when someone requests the root of the handler instead of the manifest, catalog, stream etc.
	// When no value is set, it will lead to a "404 Not Found" response.
	// Default "".
	RedirectURL string
	// Flag for indicating whether you want to expose URL handlers for the Go profiler.
	// The URLs are be the standard ones: "/debug/pprof/...".
	// Default false.
	Profiling bool
	// Flag for indicating whether you want to collect and expose Prometheus metrics.
	// The URL is the standard one: "/metrics".
	// There's no credentials required for accessing it. If you expose deflix-stremio to the public,
	// you might want to protect the metrics route in your reverse proxy.
	// Default false.
	Metrics bool
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
	// Flag for indicating whether user data is Base64-encoded.
	// As the user data is in the URL it needs to be the URL-safe Base64 encoding described in RFC 4648.
	// When true, go-stremio first decodes the value before passing or unmarshalling it.
	// Default false.
	UserDataIsBase64 bool
	// Flag for indicating whether to look up the movie / TV show name by its IMDb ID and put it into the context.
	// Only works for stream requests.
	// Default false.
	PutMetaInContext bool
	// Flag for indicating whether to include the movie / TV show name (and year) in the request log.
	// Only works for stream requests.
	// Default false.
	LogMediaName bool
	// Meta client for fetching movie and TV show info.
	// Only relevant when using PutMetaInContext or LogMediaName.
	// You can set it if you have already created one to share its in-memory cache for example,
	// or leave it empty to let go-stremio create a client that fetches metadata from Stremio's Cinemeta remote addon.
	MetaClient MetaFetcher
	// Timeout for requests to Cinemeta.
	// Only relevant when using PutMetaInContext or LogMediaName.
	// Only required when not setting a MetaClient in the options already.
	// Note that each response is cached for 30 days, so waiting a bit once per movie / TV show per 30 days is acceptable.
	// Default 2 seconds.
	CinemetaTimeout time.Duration
	// "File system" with HTML files that will be served for the "/configure" endpoint.
	// Typically an `http.Dir`, which you can simply create with `http.Dir("/path/to/html/files")`.
	// For using it with Go's embedding feature, you can either use `http.FS(embedFS)` directly,
	// or if the directory doesn't match the URL path you can use `stremio.PrefixedFS`.
	// No configure endpoint will be created if this is nil, so you can add a custom one.
	// Default nil.
	ConfigureHTMLfs http.FileSystem
	// Regex for accepted stream IDs.
	// Even when setting the "tt" prefix in the manifest to only allow IMDb IDs, some clients still send stream requests for completely different IDs,
	// potentially leading to your handlers being triggered and executing some logic before than failing due to the bad ID.
	// With this regex you can make sure your handlers are only called for valid IDs. An empty value will lead to your handlers being called for any ID.
	// URL-escaped values in the ID will be unescaped before matching.
	// IMDb example: "^tt\\d{7,8}$" or `^tt\d{7,8}$`
	// Default "".
	StreamIDregex string
}

// DefaultOptions is an Options object with default values.
// For fields that aren't set here the zero value is the default value.
var DefaultOptions = Options{
	BindAddr:        "localhost",
	Port:            8080,
	LoggingLevel:    "info",
	LogEncoding:     "console",
	CinemetaTimeout: 2 * time.Second,
}
