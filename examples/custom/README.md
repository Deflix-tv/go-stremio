# Custom addon example

This example is a more complex example showing some advanced features of go-stremio, including custom middleware and endpoints.

- Its endpoints can't be accessed without user data in the request URL
  - The user data type is registered so that go-stremio passes an object of the struct to the handler and no additional decoding or JSON unmarshalling is required
- It uses a custom "auth" middleware to block unauthorized requests to selected endpoints
  - This showcases how user data can be used when go-stremio doesn't pass an already decoded and unmarshalled object to the method
  - This showcases how to use the `fiber.Ctx` object in a custom middleware
- It contains a `web` directory with an `index.html` file which is served when visiting the `/configure` endpoint in a browser.
  - The page allows a user to 1. enter their credentials and 2. select their favorite stream type (torrent or HTTP)
- It uses a custom middleware for the `/stream` endpoint which logs the movie name a user is asking for
  - This showcases how a `cinemeta.Meta` object can be read from the context, thanks to the `PutMetaInContext: true` option
- It uses a manifest callback to track the number of "installations"
  - This showcases the usage of user data when passed by go-stremio.
- It uses a custom endpoint

## Run

1. `git clone https://github.com/Deflix-tv/go-stremio.git`
2. `cd ./go-stremio/examples/custom`
3. `go build`
4. Windows: `./custom.exe`; macOS and Linux: `./custom`

> Note: We don't use `go run .` here because that creates the binary in a temporary directory and then it can't access the `web` directory.

You should see some startup logs like this:

```text
2021-04-26T12:34:56+02:00       INFO    Setting up server...
2021-04-26T12:34:56+02:00       INFO    Finished setting up server
2021-04-26T12:34:56+02:00       INFO    Starting server {"address": "localhost:8080"}
```

To change the port for example you can change the `stremio.Options` that are passed to `stremio.NewAddon()`.

## Use

To generate user data with valid credentials and your stream type preference and install the addon with a click of a button you can visit <http://localhost:8080/configure> in your browser.
Valid credentials are either UserID: "123", Token: "abc" or UserID: "456", Token: "def".

You can also manually generate valid user data: Create a JSON object with valid credentals and your favorite stream type, then Base64-encode it (URL-safe). For example `{"userId":"123","token":"abc","preferredStreamType":"http"}` becomes the user data string `eyJ1c2VySWQiOiIxMjMiLCJ0b2tlbiI6ImFiYyIsInByZWZlcnJlZFN0cmVhbVR5cGUiOiJodHRwIn0`. So if you don't want to or couldn't install the addon via the `/configure` endpoint in your browser, you can paste the addon URL with the user data into Stremio's addon search field:
`http://localhost:8080/eyJ1c2VySWQiOiIxMjMiLCJ0b2tlbiI6ImFiYyIsInByZWZlcnJlZFN0cmVhbVR5cGUiOiJodHRwIn0/manifest.json`

If you paste the URL into Stremio without the user data you'll see that Stremio doesn't offer an "Install" button, but only a "Configure" one that redirects you to the `/configure` endpoint in your browser.

You can also use the user data in URLs to test the addon in the terminal:

```text
$ curl "http://localhost:8080/eyJ1c2VySWQiOiIxMjMiLCJ0b2tlbiI6ImFiYyIsInByZWZlcnJlZFN0cmVhbVR5cGUiOiJodHRwIn0/stream/movie/tt1254207.json" | jq .
{
  "streams": [
    {
      "url": "http://distribution.bbb3d.renderfarming.net/video/mp4/bbb_sunflower_1080p_30fps_normal.mp4",
      "title": "1080p (HTTP stream)"
    }
  ]
}
```

Here you can see that the streams were already filtered based on your user data (where we set our preference to HTTP streams instead of torrents).

In the server logs you'll see a line like this:

```text
2021-04-26T12:34:56+02:00       INFO    User is asking for stream       {"movie": "Big Buck Bunny"}
```

This is from the custom middleware that uses the `cinemeta.Meta` object that go-stremio injects into the context.

There's lots of other things you can test. Go ahead and do some requests against other endpoints, with or without user data, with valid or invalid credentials etc., then check the code to see how it's implemented.

Have fun and let us know in the GitHub issues if anything's not clear or doesn't work as you'd expect!
