package stremio

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/cors"
	"github.com/gofiber/fiber"
	"go.uber.org/zap"

	"github.com/deflix-tv/go-stremio/pkg/cinemeta"
)

type customMiddleware struct {
	path string
	mw   func(*fiber.Ctx)
}

func corsMiddleware() func(*fiber.Ctx) {
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
		AllowHeaders: []string{
			"Accept",
			"Accept-Language",
			"Content-Type",
			"Origin", // Not "safelisted" in the specification

			// Non-default for gorilla/handlers CORS handling
			"Accept-Encoding",
			"Content-Language", // "Safelisted" in the specification
			"X-Requested-With",
		},
		AllowMethods: []string{"GET"},
		AllowOrigins: []string{"*"},
	}
	return cors.New(config)
}

func createLoggingMiddleware(logger *zap.Logger, logIPs, logUserAgent, logMediaName, isMediaNameInContext bool, cinemetaClient *cinemeta.Client) func(*fiber.Ctx) {
	base64URLregex := "[A-Za-z0-9-_]+={0,2}"
	streamURLregex := regexp.MustCompile(`^/(` + base64URLregex + `/)?stream/(movie|series)/.+\.json(\?.*)?$`)

	return func(c *fiber.Ctx) {
		start := time.Now()

		// Logging media name only works for stream requests
		var isStream bool
		if logMediaName {
			isStream = streamURLregex.MatchString(c.Path())
		}

		// If the media name should be logged and it's not being put into the context,
		// we can start a goroutine to determine the media name here
		// and read it right before logging.
		var mediaName string
		var wg sync.WaitGroup
		if logMediaName && !isMediaNameInContext && isStream {
			wg = sync.WaitGroup{}
			wg.Add(1)

			go func() {
				t := c.Params("type", "")
				id := c.Params("id", "")
				if t == "" || id == "" {
					logger.Warn("Can't determine media type and/or IMDb ID from path parameters")
					return
				}

				var meta cinemeta.Meta
				var err error
				switch t {
				case "movie":
					meta, err = cinemetaClient.GetMovie(c.Context(), id)
					if err != nil {
						logger.Error("Couldn't get movie info from Cinemeta", zap.Error(err))
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
					meta, err = cinemetaClient.GetTVShow(c.Context(), splitID[0], season, episode)
					if err != nil {
						logger.Error("Couldn't get TV show info from Cinemeta", zap.Error(err))
						return
					}
				}
				logger.Debug("Got meta from cinemata client", zap.String("meta", fmt.Sprintf("%+v", meta)))

				mediaName = fmt.Sprintf("%v (%v)", meta.Name, meta.ReleaseInfo)
				wg.Done()
			}()
		}

		// First call the other handlers in the chain!
		c.Next()

		// Then log

		// If we should log the media name, we need to either wait for the previously started goroutine
		// or read it from the context.
		// We can wait for the wg in any case, as it immediately returns in case no goroutine was started.
		wg.Wait()
		if logMediaName && isMediaNameInContext && isStream {
			metaIface := c.Locals("meta")
			if metaIface == nil {
				logger.Error("No meta in context")
			} else if meta, ok := metaIface.(cinemeta.Meta); ok {
				mediaName = fmt.Sprintf("%v (%v)", meta.Name, meta.ReleaseInfo)
			} else {
				logger.Error("Couldn't turn meta interface value to proper object", zap.String("type", fmt.Sprintf("%T", metaIface)))
			}
		}

		duration := time.Since(start).Milliseconds()
		durationString := strconv.FormatInt(duration, 10) + "ms"

		if logMediaName {
			if mediaName == "" {
				mediaName = "?"
			}

			if !logIPs && !logUserAgent {
				logger.Info("Handled request",
					zap.Int("status", c.Fasthttp.Response.StatusCode()),
					zap.String("duration", durationString),
					zap.String("method", c.Method()),
					zap.String("url", c.OriginalURL()),
					zap.String("mediaName", mediaName))
			} else if logIPs && !logUserAgent {
				logger.Info("Handled request",
					zap.Int("status", c.Fasthttp.Response.StatusCode()),
					zap.String("duration", durationString),
					zap.String("method", c.Method()),
					zap.String("url", c.OriginalURL()),
					zap.String("ip", c.IP()),
					zap.Strings("forwardedFor", c.IPs()),
					zap.String("mediaName", mediaName))
			} else if !logIPs && logUserAgent {
				logger.Info("Handled request",
					zap.Int("status", c.Fasthttp.Response.StatusCode()),
					zap.String("duration", durationString),
					zap.String("method", c.Method()),
					zap.String("url", c.OriginalURL()),
					zap.String("userAgent", c.Get(fiber.HeaderUserAgent)),
					zap.String("mediaName", mediaName))
			} else {
				logger.Info("Handled request",
					zap.Int("status", c.Fasthttp.Response.StatusCode()),
					zap.String("duration", durationString),
					zap.String("method", c.Method()),
					zap.String("url", c.OriginalURL()),
					zap.String("ip", c.IP()),
					zap.Strings("forwardedFor", c.IPs()),
					zap.String("userAgent", c.Get(fiber.HeaderUserAgent)),
					zap.String("mediaName", mediaName))
			}
		} else {
			if !logIPs && !logUserAgent {
				logger.Info("Handled request",
					zap.Int("status", c.Fasthttp.Response.StatusCode()),
					zap.String("duration", durationString),
					zap.String("method", c.Method()),
					zap.String("url", c.OriginalURL()))
			} else if logIPs && !logUserAgent {
				logger.Info("Handled request",
					zap.Int("status", c.Fasthttp.Response.StatusCode()),
					zap.String("duration", durationString),
					zap.String("method", c.Method()),
					zap.String("url", c.OriginalURL()),
					zap.String("ip", c.IP()),
					zap.Strings("forwardedFor", c.IPs()))
			} else if !logIPs && logUserAgent {
				logger.Info("Handled request",
					zap.Int("status", c.Fasthttp.Response.StatusCode()),
					zap.String("duration", durationString),
					zap.String("method", c.Method()),
					zap.String("url", c.OriginalURL()),
					zap.String("userAgent", c.Get(fiber.HeaderUserAgent)))
			} else {
				logger.Info("Handled request",
					zap.Int("status", c.Fasthttp.Response.StatusCode()),
					zap.String("duration", durationString),
					zap.String("method", c.Method()),
					zap.String("url", c.OriginalURL()),
					zap.String("ip", c.IP()),
					zap.Strings("forwardedFor", c.IPs()),
					zap.String("userAgent", c.Get(fiber.HeaderUserAgent)))
			}
		}
	}
}

func createMetaMiddleware(cinemetaClient *cinemeta.Client, logger *zap.Logger) func(*fiber.Ctx) {
	return func(c *fiber.Ctx) {
		t := c.Params("type", "")
		id := c.Params("id", "")
		if t == "" || id == "" {
			logger.Warn("Can't determine media type and/or IMDb ID from path parameters")
			c.Next()
			return
		}

		var meta cinemeta.Meta
		var err error
		switch t {
		case "movie":
			meta, err = cinemetaClient.GetMovie(c.Context(), id)
			if err != nil {
				logger.Error("Couldn't get movie info from Cinemeta", zap.Error(err))
				c.Next()
				return
			}
		case "series":
			splitID := strings.Split(id, ":")
			if len(splitID) != 3 {
				logger.Warn("No 3 elements after splitting TV show ID by \":\"", zap.String("id", id))
				c.Next()
				return
			}
			season, err := strconv.Atoi(splitID[1])
			if err != nil {
				logger.Warn("Can't parse season as int", zap.String("season", splitID[1]))
				c.Next()
				return
			}
			episode, err := strconv.Atoi(splitID[2])
			if err != nil {
				logger.Warn("Can't parse episode as int", zap.String("episode", splitID[2]))
				c.Next()
				return
			}
			meta, err = cinemetaClient.GetTVShow(c.Context(), splitID[0], season, episode)
			if err != nil {
				logger.Error("Couldn't get TV show info from Cinemeta", zap.Error(err))
				c.Next()
				return
			}
		}
		logger.Debug("Got meta from cinemata client", zap.String("meta", fmt.Sprintf("%+v", meta)))
		c.Locals("meta", meta)

		c.Next()
	}
}
