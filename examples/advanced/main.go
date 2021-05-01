package main

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/deflix-tv/go-stremio"
	"github.com/deflix-tv/go-stremio/pkg/cinemeta"
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

		BehaviorHints: stremio.BehaviorHints{
			Configurable:          true,
			ConfigurationRequired: true,
		},
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
			URL:   "https://ftp.halifax.rwth-aachen.de/blender/demo/movies/BBB/bbb_sunflower_1080p_30fps_normal.mp4",
			Title: "1080p (HTTP stream)",
		},
	}
)

// content is our static web server content.
//go:embed web/index.html
var content embed.FS

// Dummy "DB" of users - just to demonstrate some functionality
var allowedUsers = []customer{
	{UserID: "123", Token: "abc"},
	{UserID: "456", Token: "def"},
}

// This struct will be used to decode the user-provided "user data" in each request from Stremio to this addon!
//
// For testing you can use `eyJ1c2VySWQiOiIxMjMiLCJ0b2tlbiI6ImFiYyIsInByZWZlcnJlZFN0cmVhbVR5cGUiOiJodHRwIn0` as user data in a request,
// which is the URL-safe Base64 encoded string of `{"userId":"123","token":"abc","preferredStreamType":"http"}`.
type customer struct {
	UserID              string `json:"userId"`
	Token               string `json:"token"`
	PreferredStreamType string `json:"preferredStreamType"`
}

func main() {
	// Create the logger first, so we can use it in our handlers
	logger, err := stremio.NewLogger("debug", "")
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
		// To read from the file system for each request, which makes it possible to modify the file on-the-fly, use this:
		//   ConfigureHTMLfs: http.Dir("web"),
		// But it requires the "web" directory to be located in the same directory as the executable of this addon.
		//
		// The alternative is to embed the file into the compiled binary, which makes the access faster,
		// the distribution of the addon easier (single file instead of multiple). It requires Go 1.16 though.
		// In this example we have to use the PrefixedFS wrapper so that the request to "/configure" goes to
		// "/web", as the HTTP middleware only strips the URL path prefix, but doesn't know about our directory structure.
		// If your embedded content contains the index.html directly, you can just set `http.FS(content)` here.
		ConfigureHTMLfs: &stremio.PrefixedFS{
			Prefix: "web",
			FS:     http.FS(content),
		},
	}

	// Create addon
	addon, err := stremio.NewAddon(manifest, nil, streamHandlers, options)
	if err != nil {
		logger.Fatal("Couldn't create new addon", zap.Error(err))
	}

	// Register the user data type
	addon.RegisterUserData(customer{})

	// Add a custom middleware that blocks unauthorized requests, but only for selected endpoints.
	// This allows requests to:
	// - The manifest without user data (Stremio needs that)
	// - The configure endpoint (where a user doesn't have encoded user data yet, or even with user data it doesn't matter)
	// - The health endpoint, which our service discovery or container orchestrator might need
	// Another reason is that adding it to "/" only would lead to the middleware not being able to read the userData URL parameter.
	authMiddleware := createAuthMiddleware(addon, logger)
	addon.AddMiddleware("/:userData/manifest.json", authMiddleware)
	addon.AddMiddleware("/stream", authMiddleware)
	addon.AddMiddleware("/:userData/stream", authMiddleware)
	addon.AddMiddleware("/:userData/ping", authMiddleware)

	// Add a custom middleware that logs which movie (name) a user is requesting
	addon.AddMiddleware("/:userData/stream", createMetaMiddleware(logger))

	// Add manifest callback that counts the number of "installations"
	manifestCallback := createManifestCallback(logger)
	addon.SetManifestCallback(manifestCallback)

	// Add a custom endpoint that responds to requests to /ping with "pong".
	customEndpoint := createCustomEndpoint(logger)
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
			// No need to check if userData is nil or if the conversion worked, because our custom auth middleware did that already.
			u, _ := userData.(*customer)

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

// Custom middleware that blocks unauthorized requests.
// Showcases the usage of user data when it's not passed from go-stremio.
func createAuthMiddleware(addon *stremio.Addon, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// We used "/:userData" when creating the auth middleware
		userDataString := c.Params("userData", "")
		if userDataString == "" {
			logger.Info("Someone sent a request without user data")
			return c.SendStatus(fiber.StatusUnauthorized)
		}

		// We used "/:userData" when creating the auth middleware, so we must pass that parameter name to access the custom user data.
		userData, err := addon.DecodeUserData("userData", c)
		if err != nil {
			logger.Warn("Couldn't decode user data", zap.Error(err))
			return c.SendStatus(fiber.StatusBadRequest)
		}
		u, ok := userData.(*customer)
		if !ok {
			t := fmt.Sprintf("%T", userData)
			logger.Error("Couldn't convert user data to customer object", zap.String("type", t))
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		// Empty user IDs and tokens can be rejected immediately
		if u.UserID == "" || u.Token == "" {
			return c.SendStatus(fiber.StatusUnauthorized)
		}

		// For others we don't want to leak whether a userID is true when a password was wrong, so either both are OK or the request is forbidden.
		for _, allowedUser := range allowedUsers {
			if u.UserID == allowedUser.UserID && u.Token == allowedUser.Token {
				return c.Next()
			}
		}
		return c.SendStatus(fiber.StatusForbidden)
	}
}

// Custom middleware that logs which movie (name) a user is asking for.
// Showcases the usage of meta info in the context.
func createMetaMiddleware(logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if meta, err := cinemeta.GetMetaFromContext(c.Context()); err != nil {
			if err == cinemeta.ErrNoMeta {
				logger.Warn("Meta not found in context")
			} else {
				logger.Error("Couldn't get meta from context", zap.Error(err))
			}
		} else {
			logger.Info("User is asking for stream", zap.String("movie", meta.Name))
		}

		return c.Next()
	}
}

// Manifest callback which counts the number of "installations".
// Showcases the usage of user data passed by go-stremio.
func createManifestCallback(logger *zap.Logger) stremio.ManifestCallback {
	var countNoData int64
	var countError int64
	var countOK int64

	return func(ctx context.Context, _ *stremio.Manifest, userData interface{}) int {
		// User provided no data
		if userData == nil {
			atomic.AddInt64(&countNoData, 1)
			logger.Info("Manifest called without user data", zap.Int64("sum", atomic.LoadInt64(&countNoData)))
			return fiber.StatusOK
		}

		u, ok := userData.(*customer)
		if !ok {
			t := fmt.Sprintf("%T", userData)
			logger.Error("Couldn't convert user data to customer object", zap.String("type", t))
			atomic.AddInt64(&countError, 1)
			logger.Info("Manifest called leading to an error", zap.Int64("sum", atomic.LoadInt64(&countError)))
			return fiber.StatusInternalServerError
		}

		// No need to check whether the user is allowed or not - the auth middleware already did that
		atomic.AddInt64(&countOK, 1)
		logger.Info("A user installed our addon", zap.Int64("sum", atomic.LoadInt64(&countOK)), zap.String("user", u.UserID))
		return fiber.StatusOK
	}
}

func createCustomEndpoint(logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.Info("A user called the ping endpoint")
		return c.SendString("pong")
	}
}
