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
	// We always log status, duration, method, URL
	zapFieldCount := 4
	if logIPs {
		// IP and Forwarded-For
		zapFieldCount += 2
	}
	if logUserAgent {
		zapFieldCount++
	}
	// For the media name it depends on if it's a stream request or not

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
			if meta, err := cinemeta.GetMetaFromContext(c.Context()); err != nil {
				if err == cinemeta.ErrNoMeta {
					logger.Warn("Meta not found in context")
				} else {
					logger.Error("Couldn't get meta from context", zap.Error(err))
				}
			} else {
				mediaName = fmt.Sprintf("%v (%v)", meta.Name, meta.ReleaseInfo)
			}
		}

		if logMediaName && isStream {
			zapFieldCount++
		}
		// TODO: To increase performance, don't create a new slice for every request. Use sync.Pool.
		zapFields := make([]zap.Field, zapFieldCount)

		duration := time.Since(start).Milliseconds()
		durationString := strconv.FormatInt(duration, 10) + "ms"

		zapFields[0] = zap.Int("status", c.Fasthttp.Response.StatusCode())
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
