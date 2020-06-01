package main

import (
	"log"

	"github.com/deflix-tv/stremio-addon-sdk"
)

var (
	manifest = stremio.Manifest{
		ID:          "org.myexampleaddon",
		Version:     "1.0.0",
		Description: "simple example",
		Name:        "simple example",

		Catalogs: []stremio.CatalogItem{},
		ResourceItems: []stremio.ResourceItem{
			{
				Name:  "stream",
				Types: []string{"movie"},
			},
		},
		Types:      []string{"movie"},
		IDprefixes: []string{"tt"},
	}
)

func streamHandler(id string) ([]stremio.StreamItem, error) {
	if id == "tt1254207" {
		return []stremio.StreamItem{{URL: "http://distribution.bbb3d.renderfarming.net/video/mp4/bbb_sunflower_1080p_30fps_normal.mp4"}}, nil
	}
	return nil, stremio.NotFound
}

func main() {
	streamHandlers := map[string]stremio.StreamHandler{"movie": streamHandler}

	addon, err := stremio.NewAddon(manifest, nil, streamHandlers, stremio.Options{BindAddr: "0.0.0.0", Port: 7000, LogLevel: "panic"})
	if err != nil {
		log.Fatalf("Couldn't create addon: %v", err)
	}

	addon.Run()
}
