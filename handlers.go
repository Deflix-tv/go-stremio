package stremio

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

func createHealthHandler(logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("healthHandler called")

		if _, err := w.Write([]byte("OK")); err != nil {
			logger.Error("Couldn't write response", zap.Error(err))
		}
	}
}

func createManifestHandler(manifest Manifest, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("manifestHandler called")

		resBody, err := json.Marshal(manifest)
		if err != nil {
			logger.Error("Couldn't marshal manifest", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		logger.Debug("Responding", zap.ByteString("body", resBody))
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(resBody); err != nil {
			logger.Error("Couldn't write response", zap.Error(err))
		}
	}
}

func createCatalogHandler(catalogHandlers map[string]CatalogHandler, cacheAge time.Duration, cachePublic, handleEtag bool, logger *zap.Logger) http.HandlerFunc {
	handlers := make(map[string]handler, len(catalogHandlers))
	for k, v := range catalogHandlers {
		handlers[k] = func(id string) (interface{}, error) {
			return v(id)
		}
	}
	return createHandler("catalog", handlers, []byte("metas"), cacheAge, cachePublic, handleEtag, logger)
}

func createStreamHandler(streamHandlers map[string]StreamHandler, cacheAge time.Duration, cachePublic, handleEtag bool, logger *zap.Logger) http.HandlerFunc {
	handlers := make(map[string]handler, len(streamHandlers))
	for k, v := range streamHandlers {
		handlers[k] = func(id string) (interface{}, error) {
			return v(id)
		}
	}
	return createHandler("stream", handlers, []byte("streams"), cacheAge, cachePublic, handleEtag, logger)
}

type handler func(id string) (interface{}, error)

func createHandler(handlerName string, handlers map[string]handler, jsonArrayKey []byte, cacheAge time.Duration, cachePublic, handleEtag bool, logger *zap.Logger) http.HandlerFunc {
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

	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug(handlerLogMsg)

		params := mux.Vars(r)
		requestedType := params["type"]
		requestedID := params["id"]

		zapLogType, zapLogID := zap.String("requestedType", requestedType), zap.String("requestedID", requestedID)

		// Check if we have a handler for the type
		handler, ok := handlers[requestedType]
		if !ok {
			logger.Warn("Got request for unhandled type; returning 404")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		res, err := handler(requestedID)
		if err != nil {
			switch err {
			case NotFound:
				logger.Warn("Got request for unhandled media ID; returning 404")
				w.WriteHeader(http.StatusNotFound)
			default:
				logger.Error("Addon returned error", zap.Error(err), zapLogType, zapLogID)
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}

		resBody, err := json.Marshal(res)
		if err != nil {
			logger.Error("Couldn't marshal response", zap.Error(err), zapLogType, zapLogID)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Handle ETag
		var eTag string
		if handleEtag {
			hash := xxhash.Sum64(resBody)
			eTag = strconv.FormatUint(hash, 16)
			ifNoneMatch := r.Header.Get("If-None-Match")
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
				w.Header().Set("Cache-Control", cacheHeaderVal) // Required according to https://tools.ietf.org/html/rfc7232#section-4.1
				w.Header().Set("ETag", eTag)                    // We set it to make sure a client doesn't overwrite its cached ETag with an empty string or so.
				w.WriteHeader(http.StatusNotModified)
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
		w.Header().Set("Content-Type", "application/json")
		if cacheHeaderVal != "" {
			w.Header().Set("Cache-Control", cacheHeaderVal)
			if handleEtag {
				w.Header().Set("ETag", eTag)
			}
		}
		if _, err := w.Write(resBody); err != nil {
			logger.Error("Coldn't write response", zap.Error(err), zapLogType, zapLogID)
		}
	}
}

func createRootHandler(redirectURL string, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("rootHandler called")

		logger.Debug("Responding with redirect", zap.String("redirectURL", redirectURL))
		w.Header().Set("Location", redirectURL)
		w.WriteHeader(http.StatusMovedPermanently)
	}
}
