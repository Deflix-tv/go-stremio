package stremio

import (
	"fmt"
	"strconv"
	"strings"
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

func createLoggingMiddleware(logger *zap.Logger, logIPs, logUserAgent bool) func(*fiber.Ctx) {
	return func(c *fiber.Ctx) {
		start := time.Now()

		// First call the other handlers in the chain!
		c.Next()

		// Then log

		duration := time.Since(start).Milliseconds()
		durationString := strconv.FormatInt(duration, 10) + "ms"

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
