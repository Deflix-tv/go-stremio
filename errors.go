package stremio

import (
	"errors"
)

var (
	// NotFound signals that the catalog/meta/stream was not found.
	// It leads to a "404 Not Found" response.
	NotFound = errors.New("Not found")
)
