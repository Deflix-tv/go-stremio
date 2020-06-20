package stremio

import (
	"strconv"
	"time"

	"github.com/gofiber/cors"
	"github.com/gofiber/fiber"
	"go.uber.org/zap"
)

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

func createLoggingMiddleware(logRequests bool, logger *zap.Logger) func(*fiber.Ctx) {
	if logRequests {
		return func(c *fiber.Ctx) {
			start := time.Now()
			// First call the other handlers in the chain!
			c.Next()
			// Then log
			duration := time.Since(start).Milliseconds()
			durationString := strconv.FormatInt(duration, 10) + "ms"

			logger.Info("Handled request",
				zap.String("method", c.Method()),
				zap.String("url", c.OriginalURL()),
				zap.String("ip", c.IP()),
				zap.Strings("forwardedFor", c.IPs()),
				zap.String("userAgent", c.Get(fiber.HeaderUserAgent)),
				zap.String("duration", durationString))
		}
	}
	return func(c *fiber.Ctx) {
		c.Next()
	}
}
