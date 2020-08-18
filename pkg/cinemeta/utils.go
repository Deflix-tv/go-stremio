package cinemeta

import (
	"context"
	"errors"
	"fmt"
)

var ErrNoMeta = errors.New("no meta in context")

// GetMetaFromContext returns the Meta object that's stored in the context.
// It returns an error if no meta was found in the context or the value found isn't of type Meta.
// The former one is ErrNoMeta which acts as sentinel error so you can check for it.
func GetMetaFromContext(ctx context.Context) (Meta, error) {
	metaIface := ctx.Value("meta")
	if metaIface == nil {
		return Meta{}, ErrNoMeta
	} else if meta, ok := metaIface.(Meta); ok {
		return meta, nil
	} else {
		return Meta{}, fmt.Errorf("couldn't turn meta interface value to proper object: type is %T", metaIface)
	}
}
