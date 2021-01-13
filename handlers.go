package stremio

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type customEndpoint struct {
	method  string
	path    string
	handler fiber.Handler
}

func createHealthHandler(logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.Debug("healthHandler called")
		return c.SendString("OK")
	}
}

func createManifestHandler(manifest Manifest, logger *zap.Logger, manifestCallback ManifestCallback, userDataType reflect.Type, userDataIsBase64 bool) fiber.Handler {
	// When there's user data we want Stremio to show the "Install" button, which it only does when "configurationRequired" is false.
	// To not change the boolean value of the manifest object on the fly and thus mess with a single object across concurrent goroutines, we copy it and return two different objects.
	// Note that this manifest copy has some values shallowly copied, but `BehaviorHints.ConfigurationRequired` is a simple type and thus a real copy.
	configuredManifest := manifest
	configuredManifest.BehaviorHints.ConfigurationRequired = false

	manifestBody, err := json.Marshal(manifest)
	if err != nil {
		logger.Fatal("Couldn't marshal manifest", zap.Error(err))
	}
	configuredManifestBody, err := json.Marshal(configuredManifest)
	if err != nil {
		logger.Fatal("Couldn't marshal configured manifest", zap.Error(err))
	}

	return func(c *fiber.Ctx) error {
		logger.Debug("manifestHandler called")

		// First call the callback so the SDK user can prevent further processing
		var userData interface{}
		userDataString := c.Params("userData")
		configured := false
		if userDataString == "" {
			if userDataType == nil {
				userData = ""
			} else {
				userData = nil
			}
		} else {
			configured = true
			if userDataType == nil {
				userData = userDataString
			} else {
				var err error
				if userData, err = decodeUserData(userDataString, userDataType, logger, userDataIsBase64); err != nil {
					return c.SendStatus(fiber.StatusBadRequest)
				}
			}
		}
		if manifestCallback != nil {
			manifestClone := manifest.clone()
			if status := manifestCallback(c.Context(), &manifestClone, userData); status >= 400 {
				return c.SendStatus(status)
			}
			// Similar to what we do before returning this handler func, we need to set `ConfigurationRequired` to false so that Stremio shows an install button at all
			if configured {
				manifestClone.BehaviorHints.ConfigurationRequired = false
			}
			// Probably no performance gain when checking deep equality of original vs cloned manifest to skip potentially unnecessary JSON encoding.
			clonedManifestBody, err := json.Marshal(manifestClone)
			if err != nil {
				logger.Fatal("Couldn't marshal cloned manifest", zap.Error(err))
			}
			logger.Debug("Responding", zap.ByteString("body", clonedManifestBody))
			c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
			return c.Send(clonedManifestBody)
		}

		if configured {
			logger.Debug("Responding", zap.ByteString("body", configuredManifestBody))
			c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
			return c.Send(configuredManifestBody)
		} else {
			logger.Debug("Responding", zap.ByteString("body", manifestBody))
			c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
			return c.Send(manifestBody)
		}
	}
}

func createCatalogHandler(catalogHandlers map[string]CatalogHandler, cacheAge time.Duration, cachePublic, handleEtag bool, logger *zap.Logger, userDataType reflect.Type, userDataIsBase64 bool) fiber.Handler {
	handlers := make(map[string]handler, len(catalogHandlers))
	for k, v := range catalogHandlers {
		handlers[k] = func(ctx context.Context, id string, userData interface{}) (interface{}, error) {
			return v(ctx, id, userData)
		}
	}
	return createHandler("catalog", handlers, []byte("metas"), cacheAge, cachePublic, handleEtag, logger, userDataType, userDataIsBase64)
}

func createStreamHandler(streamHandlers map[string]StreamHandler, cacheAge time.Duration, cachePublic, handleEtag bool, logger *zap.Logger, userDataType reflect.Type, userDataIsBase64 bool) fiber.Handler {
	handlers := make(map[string]handler, len(streamHandlers))
	for k, v := range streamHandlers {
		handlers[k] = func(ctx context.Context, id string, userData interface{}) (interface{}, error) {
			return v(ctx, id, userData)
		}
	}
	return createHandler("stream", handlers, []byte("streams"), cacheAge, cachePublic, handleEtag, logger, userDataType, userDataIsBase64)
}

// Common handler (same signature as both catalog and stream handler)
type handler func(ctx context.Context, id string, userData interface{}) (interface{}, error)

func createHandler(handlerName string, handlers map[string]handler, jsonArrayKey []byte, cacheAge time.Duration, cachePublic, handleEtag bool, logger *zap.Logger, userDataType reflect.Type, userDataIsBase64 bool) fiber.Handler {
	handlerName = handlerName + "Handler"
	handlerLogMsg := handlerName + " called"

	var cacheHeaderVal string
	if cacheAge != 0 {
		cacheAgeSeconds := strconv.FormatFloat(math.Round(cacheAge.Seconds()), 'f', 0, 64)
		cacheHeaderVal = "max-age=" + cacheAgeSeconds
		if cachePublic {
			cacheHeaderVal += ", public"
		} else {
			cacheHeaderVal += ", private"
		}
	}

	logger = logger.With(zap.String("handler", handlerName))

	return func(c *fiber.Ctx) error {
		logger.Debug(handlerLogMsg)

		requestedType := c.Params("type")
		requestedID := c.Params("id")
		requestedID, err := url.PathUnescape(requestedID)
		if err != nil {
			logger.Error("Requested ID couldn't be unescaped", zap.String("requestedID", requestedID))
			return c.SendStatus(fiber.StatusBadRequest)
		}

		zapLogType, zapLogID := zap.String("requestedType", requestedType), zap.String("requestedID", requestedID)

		// Check if we have a handler for the type
		handler, ok := handlers[requestedType]
		if !ok {
			logger.Warn("Got request for unhandled type; returning 404")
			return c.SendStatus(fiber.StatusNotFound)
		}

		// Decode user data
		var userData interface{}
		userDataString := c.Params("userData")
		if userDataType == nil {
			userData = userDataString
		} else if userDataString == "" {
			userData = nil
		} else {
			var err error
			if userData, err = decodeUserData(userDataString, userDataType, logger, userDataIsBase64); err != nil {
				return c.SendStatus(fiber.StatusBadRequest)
			}
		}

		res, err := handler(c.Context(), requestedID, userData)
		if err != nil {
			switch err {
			case NotFound:
				logger.Warn("Got request for unhandled media ID; returning 404")
				return c.SendStatus(fiber.StatusNotFound)
			case BadRequest:
				logger.Warn("Got bad request; returning 400")
				return c.SendStatus(fiber.StatusBadRequest)
			default:
				logger.Error("Addon returned error", zap.Error(err), zapLogType, zapLogID)
				return c.SendStatus(fiber.StatusInternalServerError)
			}
		}

		resBody, err := json.Marshal(res)
		if err != nil {
			logger.Error("Couldn't marshal response", zap.Error(err), zapLogType, zapLogID)
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		// Handle ETag
		var eTag string
		if handleEtag {
			hash := xxhash.Sum64(resBody)
			eTag = strconv.FormatUint(hash, 16)
			ifNoneMatch := c.Get("If-None-Match")
			zapLogIfNoneMatch, zapLogETagServer := zap.String("If-None-Match", ifNoneMatch), zap.String("ETag", eTag)
			modified := false
			if ifNoneMatch == "*" {
				logger.Debug("If-None-Match is \"*\", responding with 304", zapLogIfNoneMatch, zapLogETagServer, zapLogType, zapLogID)
			} else if ifNoneMatch != eTag {
				logger.Debug("If-None-Match != ETag", zapLogIfNoneMatch, zapLogETagServer, zapLogType, zapLogID)
				modified = true
			} else {
				logger.Debug("ETag matches, responding with 304", zapLogIfNoneMatch, zapLogETagServer, zapLogType, zapLogID)
			}
			if !modified {
				c.Set(fiber.HeaderCacheControl, cacheHeaderVal) // Required according to https://tools.ietf.org/html/rfc7232#section-4.1
				c.Set(fiber.HeaderETag, eTag)                   // We set it to make sure a client doesn't overwrite its cached ETag with an empty string or so.
				return c.SendStatus(fiber.StatusNotModified)
			}
		}

		if len(jsonArrayKey) > 0 {
			prefix := append([]byte(`{"`), jsonArrayKey...)
			prefix = append(prefix, '"', ':')
			resBody = append(prefix, resBody...)
			resBody = append(resBody, '}')
		}

		logger.Debug("Responding", zap.ByteString("body", resBody), zapLogType, zapLogID)
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		if cacheHeaderVal != "" {
			c.Set(fiber.HeaderCacheControl, cacheHeaderVal)
			if handleEtag {
				c.Set(fiber.HeaderETag, eTag)
			}
		}
		return c.Send(resBody)
	}
}

func createRootHandler(redirectURL string, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.Debug("rootHandler called")

		logger.Debug("Responding with redirect", zap.String("redirectURL", redirectURL))
		c.Set(fiber.HeaderLocation, redirectURL)
		return c.SendStatus(fiber.StatusMovedPermanently)
	}
}

func decodeUserData(data string, t reflect.Type, logger *zap.Logger, userDataIsBase64 bool) (interface{}, error) {
	logger.Debug("Decoding user data", zap.String("userData", data))

	var userDataDecoded []byte
	var err error
	if userDataIsBase64 {
		// Remove padding so that both Base64URL values with and without padding work.
		data = strings.TrimSuffix(data, "=")
		userDataDecoded, err = base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(data)
	} else {
		var userDataDecodedString string
		userDataDecodedString, err = url.PathUnescape(data)
		userDataDecoded = []byte(userDataDecodedString)
	}
	if err != nil {
		// We use WARN instead of ERROR because it's most likely an *encoding* error on the client side
		logger.Warn("Couldn't decode user data", zap.Error(err))
		return nil, err
	}

	userData := reflect.New(t).Interface()
	if err := json.Unmarshal(userDataDecoded, userData); err != nil {
		logger.Warn("Couldn't unmarshal user data", zap.Error(err))
		return nil, err
	}
	logger.Debug("Decoded user data", zap.String("userData", fmt.Sprintf("%+v", userData)))
	return userData, nil
}
