package stremio

import (
	"errors"
)

var (
	// BadRequest signals that the client sent a bad request.
	// It leads to a "400 Bad Request" response.
	BadRequest = errors.New("Bad request")
	// NotFound signals that the catalog/meta/stream was not found.
	// It leads to a "404 Not Found" response.
	NotFound = errors.New("Not found")
)
