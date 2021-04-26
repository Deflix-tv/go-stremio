# Stream addon example

This example is a minimal addon for *streams*. In `main()`, it sets up a stream handler, creates an options object for some HTTP cache handling, creates the addon and starts it.

The stream handler is set to handle requests for the type "movie" (Stremio passes media types in each stream request). In the handler it checks the IMDb ID and returns two streams per movie - one HTTP stream and one torrent stream.

## Run

1. `git clone https://github.com/Deflix-tv/go-stremio.git`
2. `cd ./go-stremio/examples/stream`
3. `go run .`

You should see some startup logs like this:

```text
2021-04-26T12:34:56+02:00       INFO    Setting up server...
2021-04-26T12:34:56+02:00       INFO    Finished setting up server
2021-04-26T12:34:56+02:00       INFO    Starting server {"address": "localhost:8080"}
```

To change the port for example you can change the `stremio.Options` that are passed to `stremio.NewAddon()`.

## Use

In Stremio you can add the addon by pasting the URL `http://localhost:8080/manifest.json` into the addon search field.

You can also test the addon in the terminal:

```text
$ curl "http://localhost:8080/stream/movie/tt1254207.json" | jq .
{
  "streams": [
    {
      "infoHash": "dd8255ecdc7ca55fb0bbf81323d87062db1f6d1c",
      "title": "1080p (torrent)",
      "fileIdx": 1
    },
    {
      "url": "http://distribution.bbb3d.renderfarming.net/video/mp4/bbb_sunflower_1080p_30fps_normal.mp4",
      "title": "1080p (HTTP stream)"
    }
  ]
}
```
