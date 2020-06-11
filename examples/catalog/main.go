package main

import (
	"log"
	"time"

	"github.com/deflix-tv/go-stremio"
)

var (
	version = "0.1.0"

	manifest = stremio.Manifest{
		ID:          "com.example.blender-catalog",
		Name:        "Blender movie catalog",
		Description: "Catalog addon for free movies that were made with Blender",
		Version:     version,

		ResourceItems: []stremio.ResourceItem{
			{
				Name: "catalog",
			},
		},
		Types:    []string{"movie"},
		Catalogs: catalogs,
	}

	catalogs = []stremio.CatalogItem{
		{
			Type: "movie",
			ID:   "blender",
			Name: "Free movies made with Blender",
		},
	}
)

func main() {
	catalogHandlers := map[string]stremio.CatalogHandler{"movie": movieHandler}

	// We want clients and proxies to cache the response for 24 hours
	// and upon request with the same hash we only return a 304 Not Modified.
	options := stremio.Options{
		CacheAgeCatalogs:    24 * time.Hour,
		CachePublicCatalogs: true,
		HandleEtagCatalogs:  true,
	}

	addon, err := stremio.NewAddon(manifest, catalogHandlers, nil, options)
	if err != nil {
		log.Fatalf("Couldn't create addon: %v", err)
	}

	addon.Run()
}

func movieHandler(id string) ([]stremio.MetaPreviewItem, error) {
	if id != "blender" {
		return nil, stremio.NotFound
	}
	return []stremio.MetaPreviewItem{
		{
			ID:     "tt1254207",
			Type:   "movie",
			Name:   "Big Buck Bunny",
			Poster: "https://upload.wikimedia.org/wikipedia/commons/thumb/c/c5/Big_buck_bunny_poster_big.jpg/339px-Big_buck_bunny_poster_big.jpg",
		},
		{
			ID:     "tt1727587",
			Type:   "movie",
			Name:   "Sintel",
			Poster: "https://images.metahub.space/poster/small/tt1727587/img",
		},
	}, nil
}
