package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/deflix-tv/go-stremio"
	"github.com/gofiber/fiber"
)

var (
	version = "0.1.0"

	manifest = stremio.Manifest{
		ID:          "com.example.blender-streams-custom",
		Name:        "Custom Blender movie streams",
		Description: "Stream addon for free movies that were made with Blender, customizable via user data",
		Version:     version,

		ResourceItems: []stremio.ResourceItem{
			{
				Name:  "stream",
				Types: []string{"movie"},
			},
		},
		Types: []string{"movie"},
		// An empty slice is required for serializing to a JSON that Stremio expects
		Catalogs: []stremio.CatalogItem{},

		IDprefixes: []string{"tt"},
	}

	streams = []stremio.StreamItem{
		// Torrent stream
		{
			InfoHash:  "dd8255ecdc7ca55fb0bbf81323d87062db1f6d1c",
			Title:     "1080p (torrent)",
			FileIndex: 1,
		},
		// HTTP stream
		{
			URL:   "http://distribution.bbb3d.renderfarming.net/video/mp4/bbb_sunflower_1080p_30fps_normal.mp4",
			Title: "1080p (HTTP stream)",
		},
	}
)

// Dummy "DB" of users - just to demonstrate some functionality
var allowedUsers = []customer{
	{UserID: "123", Token: "abc"},
	{UserID: "456", Token: "def"},
}

// This struct will be used to decode the user-provided "user data" in each request from Stremio to this addon!
//
// For testing you can use `eyJ1c2VySWQiOiIxMjMiLCJ0b2tlbiI6ImFiYyIsInByZWZlcnJlZFN0cmVhbVR5cGUiOiJodHRwIn0=` as user data in a request,
// which is the URL-safe Base64 encoded string of `{"userId":"123","token":"abc","preferredStreamType":"http"}`.
type customer struct {
	UserID              string `json:"userId"`
	Token               string `json:"token"`
	PreferredStreamType string `json:"preferredStreamType"`
}

func main() {
	streamHandlers := map[string]stremio.StreamHandler{"movie": movieHandler}
	options := stremio.Options{
		UserDataIsBase64: true,
	}
	addon, err := stremio.NewAddon(manifest, manifestCallback, nil, streamHandlers, options)
	if err != nil {
		panic(err)
	}

	// Register the user data type
	addon.RegisterUserData(customer{})

	// Add a custom middleware that counts the number of requests per route and regularly prints it
	addon.AddMiddleware("/", createCustomMiddleware())

	// Add a custom endpoint that responds to requests to /ping with "pong".
	addon.AddEndpoint("GET", "/ping", func(c *fiber.Ctx) {
		c.SendString("pong")
	})

	addon.Run()
}

// Manifest callback which prevents installations by unknown users and logs successful installations
func manifestCallback(userData interface{}) int {
	// User provided no data
	if userData == nil {
		return fiber.StatusUnauthorized
	}

	u, ok := userData.(*customer)
	if !ok {
		log.Printf("Couldn't convert user data to customer object. Type: %T", userData)
		return fiber.StatusInternalServerError
	}

	for _, allowedUser := range allowedUsers {
		if u.UserID == allowedUser.UserID && u.Token == allowedUser.Token {
			log.Printf("User %v installed our addon!", u.UserID)
			return fiber.StatusOK
		}
	}
	// User provided data, but didn't match any of the allowed users
	return fiber.StatusForbidden
}

func createCustomMiddleware() func(c *fiber.Ctx) {
	stats := map[string]int{}
	lock := sync.Mutex{}

	// Print route stats every 10 seconds
	go func() {
		for {
			time.Sleep(10 * time.Second)
			lock.Lock()
			log.Printf("Route stats: %+v", stats)
			lock.Unlock()
		}
	}()

	return func(c *fiber.Ctx) {
		route := c.OriginalURL()
		lock.Lock()
		count, ok := stats[route]
		if !ok {
			count = 0
		}
		stats[route] = count + 1
		// We unlock manually instead of by deferring so we don't block for the whole duration
		// of the remaining request processing in other middlewares and handlers
		lock.Unlock()

		c.Next()
	}
}

func movieHandler(id string, userData interface{}) ([]stremio.StreamItem, error) {
	// We only serve Big Buck Bunny
	if id == "tt1254207" {
		// User provided no data
		if userData == nil {
			return streams, nil
		}

		u, ok := userData.(*customer)
		if !ok {
			return nil, fmt.Errorf("Couldn't convert user data to customer object. Type: %T", userData)
		}
		// Return different streams depending on the user's preference
		switch u.PreferredStreamType {
		case "torrent":
			return []stremio.StreamItem{streams[0]}, nil
		case "http":
			return []stremio.StreamItem{streams[1]}, nil
		default:
			return streams, nil
		}
	}
	return nil, stremio.NotFound
}
