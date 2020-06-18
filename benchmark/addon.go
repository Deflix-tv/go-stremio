package main

import (
	"github.com/deflix-tv/go-stremio"
)

var (
	manifest = stremio.Manifest{
		ID:      "org.myexampleaddon",
		Version: "1.0.0",

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

	addon, err := stremio.NewAddon(manifest, nil, streamHandlers, stremio.Options{BindAddr: "0.0.0.0", Port: 7000, DisableRequestLogging: true})
	if err != nil {
		addon.Logger().Sugar().Fatalf("Couldn't create addon: %v", err)
	}

	addon.Run()
}
