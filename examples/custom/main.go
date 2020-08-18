package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/deflix-tv/go-stremio"
	"github.com/deflix-tv/go-stremio/pkg/cinemeta"
	"github.com/gofiber/fiber"
	"go.uber.org/zap"
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
	// Create the logger first, so we can use it in our handlers
	logger, err := stremio.NewLogger("debug")
	if err != nil {
		panic(err)
	}

	// Create movie handler that uses the logger we previously created
	movieHandler := createMovieHandler(logger)
	// Let the movieHandler handle the "movie" type
	streamHandlers := map[string]stremio.StreamHandler{"movie": movieHandler}

	options := stremio.Options{
		// We already have a logger
		Logger: logger,
		// Our addon uses Base64 encoded user data
		UserDataIsBase64: true,
		// We want to access the cinemeta.Meta from the context
		PutMetaInContext: true,
	}

	// Create addon
	addon, err := stremio.NewAddon(manifest, nil, streamHandlers, options)
	if err != nil {
		logger.Fatal("Couldn't create new addon", zap.Error(err))
	}

	// Register the user data type
	addon.RegisterUserData(customer{})

	// Add a custom middleware that counts the number of requests per route and regularly prints it
	customMiddleware := createCustomMiddleware(logger)
	addon.AddMiddleware("/", customMiddleware)

	// Add manifest callback that uses the logger we previously created
	manifestCallback := createManifestCallback(logger)
	addon.SetManifestCallback(manifestCallback)

	// Add a custom endpoint that responds to requests to /ping with "pong".
	customEndpoint := createCustomEndpoint(addon, logger)
	addon.AddEndpoint("GET", "/:userData/ping", customEndpoint)

	// The stopping channel allows us to react on the addon being shutdown, for example because of a system signal received from Ctrl+C or `docker stop`
	stoppingChan := make(chan bool, 1)
	go func() {
		<-stoppingChan
		logger.Info("Addon stopping")
	}()
	addon.Run(stoppingChan)
}

func createMovieHandler(logger *zap.Logger) stremio.StreamHandler {
	return func(ctx context.Context, id string, userData interface{}) ([]stremio.StreamItem, error) {
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
			logger.Info("User requested stream", zap.String("userID", u.UserID))
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
}

func createCustomMiddleware(logger *zap.Logger) fiber.Handler {
	stats := map[string]int{}
	lock := sync.Mutex{}

	// Print route stats every 10 seconds
	go func() {
		for {
			time.Sleep(10 * time.Second)
			lock.Lock()
			statsString := fmt.Sprintf("%+v", stats)
			logger.Info("Route stats", zap.String("stats", statsString))
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

		metaIface := c.Locals("meta")
		if metaIface == nil {
			logger.Error("No meta in context")
		} else if meta, ok := metaIface.(cinemeta.Meta); ok {
			logger.Info("User is asking for stream", zap.String("movie", meta.Name))
		} else {
			logger.Error("Couldn't turn meta interface value to proper object", zap.String("type", fmt.Sprintf("%T", metaIface)))
		}

		c.Next()
	}
}

// Manifest callback which prevents installations by unknown users and logs successful installations
func createManifestCallback(logger *zap.Logger) stremio.ManifestCallback {
	return func(ctx context.Context, userData interface{}) int {
		// User provided no data
		if userData == nil {
			return fiber.StatusUnauthorized
		}

		u, ok := userData.(*customer)
		if !ok {
			t := fmt.Sprintf("%T", userData)
			logger.Error("Couldn't convert user data to customer object", zap.String("type", t))
			return fiber.StatusInternalServerError
		}

		for _, allowedUser := range allowedUsers {
			if u.UserID == allowedUser.UserID && u.Token == allowedUser.Token {
				logger.Info("A user installed our addon", zap.String("user", u.UserID))
				return fiber.StatusOK
			}
		}
		// User provided data, but didn't match any of the allowed users
		return fiber.StatusForbidden
	}
}

func createCustomEndpoint(addon *stremio.Addon, logger *zap.Logger) func(*fiber.Ctx) {
	return func(c *fiber.Ctx) {
		// We used "/:userData" when creating the custom endpoint, so we must pass that parameter name to access the custom user data.
		userData, err := addon.DecodeUserData("userData", c)
		if err != nil {
			logger.Warn("Couldn't decode user data", zap.Error(err))
			c.Status(fiber.StatusBadRequest)
			return
		}
		u, ok := userData.(*customer)
		if !ok {
			t := fmt.Sprintf("%T", userData)
			logger.Error("Couldn't convert user data to customer object", zap.String("type", t))
			c.Status(fiber.StatusInternalServerError)
			return
		}
		logger.Info("A user called the ping endpoint", zap.String("user", u.UserID))

		c.SendString("pong")
	}
}
