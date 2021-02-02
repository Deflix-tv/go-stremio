package stremio

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/deflix-tv/go-stremio/pkg/cinemeta"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"go.uber.org/zap"
)

type customMiddleware struct {
	path string
	mw   fiber.Handler
}

func createLoggingMiddleware(logger *zap.Logger, logIPs, logUserAgent, logMediaName bool, requiresUserData bool) fiber.Handler {
	// We always log status, duration, method, URL
	zapFieldCount := 4
	if logIPs {
		// IP and Forwarded-For
		zapFieldCount += 2
	}
	if logUserAgent {
		zapFieldCount++
	}

	return func(c *fiber.Ctx) error {
		start := time.Now()

		// First call the other handlers in the chain!
		if err := c.Next(); err != nil {
			logger.Error("Received error from next middleware or handler in logging middleware", zap.Error(err))
		}

		// Then log

		isStream := c.Locals("isStream") != nil

		// Get meta from context - the meta middleware put it there.
		// We ignore ErrNoMeta here, because actual issues are logged by the meta middleware already, and here we'd have to check for things like "is config required but not set", "is the ID bad and the ID matcher was used" which are all valid cases to not have meta in the context.
		var mediaName string
		if logMediaName && isStream {
			if meta, err := cinemeta.GetMetaFromContext(c.Context()); err != nil && err != cinemeta.ErrNoMeta {
				logger.Error("Couldn't get meta from context", zap.Error(err))
			} else if err != cinemeta.ErrNoMeta {
				mediaName = fmt.Sprintf("%v (%v)", meta.Name, meta.ReleaseInfo)
			}
		}

		var zapFields []zap.Field
		// TODO: To increase performance, don't create a new slice for every request. Use sync.Pool.
		if logMediaName && isStream {
			zapFields = make([]zap.Field, zapFieldCount+1)
		} else {
			zapFields = make([]zap.Field, zapFieldCount)
		}

		duration := time.Since(start).Milliseconds()
		durationString := strconv.FormatInt(duration, 10) + "ms"

		zapFields[0] = zap.Int("status", c.Response().StatusCode())
		zapFields[1] = zap.String("duration", durationString)
		zapFields[2] = zap.String("method", c.Method())
		zapFields[3] = zap.String("url", c.OriginalURL())
		if logIPs {
			zapFields[4] = zap.String("ip", c.IP())
			zapFields[5] = zap.Strings("forwardedFor", c.IPs())
		}
		if logUserAgent {
			if !logIPs {
				zapFields[4] = zap.String("userAgent", c.Get(fiber.HeaderUserAgent))
			} else {
				zapFields[6] = zap.String("userAgent", c.Get(fiber.HeaderUserAgent))
			}
		}
		if logMediaName && isStream {
			if mediaName == "" {
				mediaName = "?"
			}
			if !logIPs && !logUserAgent {
				zapFields[4] = zap.String("mediaName", mediaName)
			} else if !logIPs && logUserAgent {
				zapFields[5] = zap.String("mediaName", mediaName)
			} else if logIPs && !logUserAgent {
				zapFields[6] = zap.String("mediaName", mediaName)
			} else {
				zapFields[7] = zap.String("mediaName", mediaName)
			}
		}

		logger.Info("Handled request", zapFields...)
		return nil
	}
}

func createMetricsMiddleware() fiber.Handler {
	// Total number of errors from downstream handlers in the metrics middleware
	errCounter := metrics.NewCounter("downstream_handlers_errors_total")

	manifestRegex := regexp.MustCompile("^/.*/manifest.json$")
	catalogRegex := regexp.MustCompile(`^/.*/catalog/.*/.*\.json`)
	streamRegex := regexp.MustCompile(`^/.*/stream/.*/.*\.json`)

	return func(c *fiber.Ctx) error {
		if err := c.Next(); err != nil {
			errCounter.Inc()
			return err
		}

		path := c.Path()
		var endpoint string
		switch path {
		case "/":
			endpoint = "root"
		case "/manifest.json":
			endpoint = "manifest"
		case "/configure":
			endpoint = "configure"
		case "/health":
			endpoint = "health"
		case "/metrics":
			endpoint = "metrics"
		}

		if endpoint == "" {
			if strings.HasPrefix(path, "/catalog") {
				endpoint = "catalog"
			} else if strings.HasPrefix(path, "/stream") {
				endpoint = "stream"
			} else if strings.HasPrefix(path, "/configure") {
				endpoint = "configure-other"
			} else if strings.HasPrefix(path, "/debug/pprof") {
				endpoint = "pprof"
			}
		}

		if endpoint == "" {
			if manifestRegex.MatchString(path) {
				endpoint = "manifest-data"
			} else if catalogRegex.MatchString(path) {
				endpoint = "catalog-data"
			} else if streamRegex.MatchString(path) {
				endpoint = "stream-data"
			}
		}

		// It would be valid for Prometheus to have an empty string as label, but it's confusing for users and makes custom legends in Grafana ugly.
		if endpoint == "" {
			endpoint = "other"
		}

		// Total number of HTTP requests.
		// With the VictoriaMetrics client library we have to use this workaround for having an equivalent of Prometheus' CounterVec,
		// see https://pkg.go.dev/github.com/VictoriaMetrics/metrics@v1.12.3#example-Counter-Vec.
		counterName := fmt.Sprintf(`http_requests_total{endpoint="%v", status="%v"}`, endpoint, c.Response().StatusCode())
		counter := metrics.GetOrCreateCounter(counterName)
		counter.Add(1)

		return nil
	}
}

func corsMiddleware() fiber.Handler {
	config := cors.Config{
		// Headers as listed by the Stremio example addon.
		//
		// According to logs an actual stream request sends these headers though:
		//   Header:map[
		// 	  Accept:[*/*]
		// 	  Accept-Encoding:[gzip, deflate, br]
		// 	  Connection:[keep-alive]
		// 	  Origin:[https://app.strem.io]
		// 	  User-Agent:[Mozilla/5.0 (Windows NT 6.2; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) QtWebEngine/5.9.9 Chrome/56.0.2924.122 Safari/537.36 StremioShell/4.4.106]
		// ]
		AllowHeaders: "Accept" +
			", Accept-Language" +
			", Content-Type" +
			", Origin" + // Not "safelisted" in the specification

			// Non-default for gorilla/handlers CORS handling
			", Accept-Encoding" +
			", Content-Language" + // "Safelisted" in the specification
			", X-Requested-With",
		AllowMethods: "GET,HEAD",
		AllowOrigins: "*",
	}
	return cors.New(config)
}

func addRouteMatcherMiddleware(app *fiber.App, requiresUserData bool, streamIDregexString string, logger *zap.Logger) {
	streamIDregex := regexp.MustCompile(streamIDregexString)
	if requiresUserData {
		// Catalog
		app.Use("/catalog/:type/:id.json", func(c *fiber.Ctx) error {
			// If user data is required but not sent, let clients know they sent a bad request.
			// That's better than responding with 404, leading to clients thinking it's a server-side error.
			return c.SendStatus(fiber.StatusBadRequest)
		})
		app.Use("/:userData/catalog/:type/:id.json", func(c *fiber.Ctx) error {
			if c.Params("type", "") == "" || c.Params("id", "") == "" {
				logger.Debug("Rejecting bad request due to missing type or ID")
				return c.SendStatus(fiber.StatusBadRequest)
			}
			c.Locals("isConfigured", true)
			return c.Next()
		})
		// Stream
		app.Use("/stream/:type/:id.json", func(c *fiber.Ctx) error {
			return c.SendStatus(fiber.StatusBadRequest)
		})
		app.Use("/:userData/stream/:type/:id.json", func(c *fiber.Ctx) error {
			id := c.Params("id", "")
			if c.Params("type", "") == "" || id == "" {
				logger.Debug("Rejecting bad request due to missing type or ID")
				return c.SendStatus(fiber.StatusBadRequest)
			}
			id, err := url.PathUnescape(id)
			if err != nil {
				logger.Warn("Couldn't unescape ID", zap.Error(err), zap.String("id", id))
				return c.SendStatus(fiber.StatusInternalServerError)
			}
			if !streamIDregex.MatchString(id) {
				logger.Debug("Rejecting bad request due to stream ID not matching the given regex")
				return c.SendStatus(fiber.StatusBadRequest)
			}
			c.Locals("isConfigured", true)
			c.Locals("isStream", true)
			return c.Next()
		})
	} else {
		// Catalog
		app.Use("/catalog/:type/:id.json", func(c *fiber.Ctx) error {
			if c.Params("type", "") == "" || c.Params("id", "") == "" {
				logger.Debug("Rejecting bad request due to missing type or ID")
				return c.SendStatus(fiber.StatusBadRequest)
			}
			c.Locals("isConfigured", true)
			return c.Next()
		})
		app.Use("/:userData/catalog/:type/:id.json", func(c *fiber.Ctx) error {
			if c.Params("type", "") == "" || c.Params("id", "") == "" {
				logger.Debug("Rejecting bad request due to missing type or ID")
				return c.SendStatus(fiber.StatusBadRequest)
			}
			c.Locals("isConfigured", true)
			return c.Next()
		})
		// Stream
		app.Use("/stream/:type/:id.json", func(c *fiber.Ctx) error {
			id := c.Params("id", "")
			if c.Params("type", "") == "" || id == "" {
				logger.Debug("Rejecting bad request due to missing type or ID")
				return c.SendStatus(fiber.StatusBadRequest)
			}
			id, err := url.PathUnescape(id)
			if err != nil {
				logger.Warn("Couldn't unescape ID", zap.Error(err), zap.String("id", id))
				return c.SendStatus(fiber.StatusInternalServerError)
			}
			if !streamIDregex.MatchString(id) {
				logger.Debug("Rejecting bad request due to stream ID not matching the given regex")
				return c.SendStatus(fiber.StatusBadRequest)
			}
			c.Locals("isStream", true)
			return c.Next()
		})
		app.Use("/:userData/stream/:type/:id.json", func(c *fiber.Ctx) error {
			id := c.Params("id", "")
			if c.Params("type", "") == "" || id == "" {
				logger.Debug("Rejecting bad request due to missing type or ID")
				return c.SendStatus(fiber.StatusBadRequest)
			}
			id, err := url.PathUnescape(id)
			if err != nil {
				logger.Warn("Couldn't unescape ID", zap.Error(err), zap.String("id", id))
				return c.SendStatus(fiber.StatusInternalServerError)
			}
			if !streamIDregex.MatchString(id) {
				logger.Debug("Rejecting bad request due to stream ID not matching the given regex")
				return c.SendStatus(fiber.StatusBadRequest)
			}
			c.Locals("isConfigured", true)
			c.Locals("isStream", true)
			return c.Next()
		})
	}
}

func createMetaMiddleware(metaClient MetaFetcher, putMetaInHandlerContext, logMediaName bool, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// If we should put the meta in the context for *handlers* we get the meta synchronously.
		// Otherwise we only need it for logging and can get the meta asynchronously.
		if putMetaInHandlerContext {
			putMetaInContext(c, metaClient, logger)
			return c.Next()
		} else if logMediaName {
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				putMetaInContext(c, metaClient, logger)
				wg.Done()
			}()
			err := c.Next()
			// Wait so that the meta is in the context when returning to the logging middleware
			wg.Wait()
			return err
		} else {
			return c.Next()
		}
	}
}

func putMetaInContext(c *fiber.Ctx, metaClient MetaFetcher, logger *zap.Logger) {
	var meta cinemeta.Meta
	var err error
	// type and id can never be empty, because that's been checked by a previous middleware
	t := c.Params("type", "")
	id := c.Params("id", "")
	id, err = url.PathUnescape(id)
	if err != nil {
		logger.Error("ID in URL parameters couldn't be unescaped", zap.String("id", id))
		return
	}

	switch t {
	case "movie":
		meta, err = metaClient.GetMovie(c.Context(), id)
		if err != nil {
			logger.Error("Couldn't get movie info with MetaFetcher", zap.Error(err))
			return
		}
	case "series":
		splitID := strings.Split(id, ":")
		if len(splitID) != 3 {
			logger.Warn("No 3 elements after splitting TV show ID by \":\"", zap.String("id", id))
			return
		}
		season, err := strconv.Atoi(splitID[1])
		if err != nil {
			logger.Warn("Can't parse season as int", zap.String("season", splitID[1]))
			return
		}
		episode, err := strconv.Atoi(splitID[2])
		if err != nil {
			logger.Warn("Can't parse episode as int", zap.String("episode", splitID[2]))
			return
		}
		meta, err = metaClient.GetTVShow(c.Context(), splitID[0], season, episode)
		if err != nil {
			logger.Error("Couldn't get TV show info with MetaFetcher", zap.Error(err))
			return
		}
	}

	logger.Debug("Got meta from cinemata client", zap.String("meta", fmt.Sprintf("%+v", meta)))
	c.Locals("meta", meta)
}
