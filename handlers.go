package stremio

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/gofiber/fiber"
	"go.uber.org/zap"
)

type customEndpoint struct {
	method  string
	path    string
	handler func(*fiber.Ctx)
}

func createHealthHandler(logger *zap.Logger) func(*fiber.Ctx) {
	return func(c *fiber.Ctx) {
		logger.Debug("healthHandler called")
		c.SendString("OK")
	}
}

func createManifestHandler(manifest Manifest, logger *zap.Logger, manifestCallback ManifestCallback, userDataType reflect.Type, userDataIsBase64 bool) func(*fiber.Ctx) {
	return func(c *fiber.Ctx) {
		logger.Debug("manifestHandler called")

		// First call the callback so the SDK user can prevent further processing
		var userData interface{}
		userDataString := c.Params("userData")
		if userDataType == nil {
			userData = userDataString
		} else if userDataString == "" {
			userData = nil
		} else {
			var err error
			if userData, err = decodeUserData(userDataString, userDataType, logger, userDataIsBase64); err != nil {
				c.Status(fiber.StatusBadRequest)
				return
			}
		}
		if manifestCallback != nil {
			if status := manifestCallback(userData); status >= 400 {
				c.Status(status)
				return
			}
		}

		resBody, err := json.Marshal(manifest)
		if err != nil {
			logger.Error("Couldn't marshal manifest", zap.Error(err))
			c.Status(fiber.StatusInternalServerError)
			return
		}

		logger.Debug("Responding", zap.ByteString("body", resBody))
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		c.SendBytes(resBody)
	}
}

func createCatalogHandler(catalogHandlers map[string]CatalogHandler, cacheAge time.Duration, cachePublic, handleEtag bool, logger *zap.Logger, userDataType reflect.Type, userDataIsBase64 bool) func(*fiber.Ctx) {
	handlers := make(map[string]handler, len(catalogHandlers))
	for k, v := range catalogHandlers {
		handlers[k] = func(id string, userData interface{}) (interface{}, error) {
			return v(id, userData)
		}
	}
	return createHandler("catalog", handlers, []byte("metas"), cacheAge, cachePublic, handleEtag, logger, userDataType, userDataIsBase64)
}

func createStreamHandler(streamHandlers map[string]StreamHandler, cacheAge time.Duration, cachePublic, handleEtag bool, logger *zap.Logger, userDataType reflect.Type, userDataIsBase64 bool) func(*fiber.Ctx) {
	handlers := make(map[string]handler, len(streamHandlers))
	for k, v := range streamHandlers {
		handlers[k] = func(id string, userData interface{}) (interface{}, error) {
			return v(id, userData)
		}
	}
	return createHandler("stream", handlers, []byte("streams"), cacheAge, cachePublic, handleEtag, logger, userDataType, userDataIsBase64)
}

type handler func(id string, userData interface{}) (interface{}, error)

func createHandler(handlerName string, handlers map[string]handler, jsonArrayKey []byte, cacheAge time.Duration, cachePublic, handleEtag bool, logger *zap.Logger, userDataType reflect.Type, userDataIsBase64 bool) func(*fiber.Ctx) {
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

	return func(c *fiber.Ctx) {
		logger.Debug(handlerLogMsg)

		requestedType := c.Params("type")
		requestedID := c.Params("id")

		zapLogType, zapLogID := zap.String("requestedType", requestedType), zap.String("requestedID", requestedID)

		// Check if we have a handler for the type
		handler, ok := handlers[requestedType]
		if !ok {
			logger.Warn("Got request for unhandled type; returning 404")
			c.Status(http.StatusNotFound)
			return
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
				c.Status(fiber.StatusBadRequest)
				return
			}
		}

		res, err := handler(requestedID, userData)
		if err != nil {
			switch err {
			case NotFound:
				logger.Warn("Got request for unhandled media ID; returning 404")
				c.Status(http.StatusNotFound)
			default:
				logger.Error("Addon returned error", zap.Error(err), zapLogType, zapLogID)
				c.Status(http.StatusInternalServerError)
			}
			return
		}

		resBody, err := json.Marshal(res)
		if err != nil {
			logger.Error("Couldn't marshal response", zap.Error(err), zapLogType, zapLogID)
			c.Status(http.StatusInternalServerError)
			return
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
				c.Status(http.StatusNotModified)
				return
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
		c.SendBytes(resBody)
	}
}

func createRootHandler(redirectURL string, logger *zap.Logger) func(*fiber.Ctx) {
	return func(c *fiber.Ctx) {
		logger.Debug("rootHandler called")

		logger.Debug("Responding with redirect", zap.String("redirectURL", redirectURL))
		c.Set(fiber.HeaderLocation, redirectURL)
		c.Status(http.StatusMovedPermanently)
	}
}

func decodeUserData(data string, t reflect.Type, logger *zap.Logger, userDataIsBase64 bool) (interface{}, error) {
	logger.Debug("Decoding user data", zap.String("userData", data))

	var userDataDecoded []byte
	var err error
	if userDataIsBase64 {
		userDataDecoded, err = base64.URLEncoding.DecodeString(data)
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
	return userData, nil
}
