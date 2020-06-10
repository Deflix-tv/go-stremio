package stremio

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	log.Trace("healthHandler called")

	if _, err := w.Write([]byte("OK")); err != nil {
		log.WithError(err).Error("Couldn't write response")
	}
}

func createManifestHandler(manifest Manifest) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Trace("manifestHandler called")

		resBody, err := json.Marshal(manifest)
		if err != nil {
			log.WithError(err).Error("Couldn't marshal manifest")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		log.WithField("body", string(resBody)).Debug("Responding")
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(resBody); err != nil {
			log.WithError(err).Error("Couldn't write response")
		}
	}
}

func createCatalogHandler(catalogHandlers map[string]CatalogHandler, cacheAge time.Duration, cachePublic bool) http.HandlerFunc {
	handlers := make(map[string]handler, len(catalogHandlers))
	for k, v := range catalogHandlers {
		handlers[k] = func(id string) (interface{}, error) {
			return v(id)
		}
	}
	return createHandler("catalog", handlers, []byte("metas"), cacheAge, cachePublic)
}

func createStreamHandler(streamHandlers map[string]StreamHandler, cacheAge time.Duration, cachePublic bool) http.HandlerFunc {
	handlers := make(map[string]handler, len(streamHandlers))
	for k, v := range streamHandlers {
		handlers[k] = func(id string) (interface{}, error) {
			return v(id)
		}
	}
	return createHandler("stream", handlers, []byte("streams"), cacheAge, cachePublic)
}

type handler func(id string) (interface{}, error)

func createHandler(handlerName string, handlers map[string]handler, jsonArrayKey []byte, cacheAge time.Duration, cachePublic bool) http.HandlerFunc {
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

	return func(w http.ResponseWriter, r *http.Request) {
		log.Tracef("%vHandler called", handlerName)

		params := mux.Vars(r)
		requestedType := params["type"]
		requestedID := params["id"]

		logger := log.WithFields(log.Fields{"handler": handlerName, "requestedType": requestedType, "requestedID": requestedID})

		// Check if we have a handler for the type
		handler, ok := handlers[requestedType]
		if !ok {
			logger.Warn("Got request for unhandled type")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		res, err := handler(requestedID)
		if err != nil {
			switch err {
			case NotFound:
				logger.WithError(err).Warn("Got request for unhandled type")
				w.WriteHeader(http.StatusNotFound)
			default:
				logger.WithError(err).Error("Got request for unhandled type")
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}

		resBody, err := json.Marshal(res)
		if err != nil {
			log.WithError(err).Error("Couldn't marshal response")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if len(jsonArrayKey) > 0 {
			prefix := append([]byte(`{"`), jsonArrayKey...)
			prefix = append(prefix, '"', ':')
			resBody = append(prefix, resBody...)
			resBody = append(resBody, '}')
		}

		log.WithField("body", string(resBody)).Debug("Responding")
		w.Header().Set("Content-Type", "application/json")
		if cacheHeaderVal != "" {
			w.Header().Set("Cache-Control", cacheHeaderVal)
		}
		if _, err := w.Write(resBody); err != nil {
			log.WithError(err).Error("Coldn't write response")
		}
	}
}

func createRootHandler(redirectURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Trace("rootHandler called")

		log.WithField("redirectURL", redirectURL).Debug("Responding with redirect")
		w.Header().Set("Location", redirectURL)
		w.WriteHeader(http.StatusMovedPermanently)
	}
}
