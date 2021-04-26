# Catalog addon example

This example is a minimal addon for *catalogs*. In `main()`, it sets up a catalog handler, creates an options object for some HTTP cache handling, creates the addon and starts it.

The catalog handler is set to handle requests for the type "movie" (Stremio passes media types in each stream request). In the handler it checks the catalog ID and only handles the catalog ID "blender". Note that this is the ID that's defined in the manifest, which is how Stremio knows which catalogs it can request from your addon. For the "blender" catalog the handler then returns metadata about both Big Bug Bunny and Sintel.

## Run

1. `git clone https://github.com/Deflix-tv/go-stremio.git`
2. `cd ./go-stremio/examples/catalog`
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
$ curl "http://localhost:8080/catalog/movie/blender.json" | jq .
{
  "metas": [
    {
      "id": "tt1254207",
      "type": "movie",
      "name": "Big Buck Bunny",
      "poster": "https://upload.wikimedia.org/wikipedia/commons/thumb/c/c5/Big_buck_bunny_poster_big.jpg/339px-Big_buck_bunny_poster_big.jpg"
    },
    {
      "id": "tt1727587",
      "type": "movie",
      "name": "Sintel",
      "poster": "https://images.metahub.space/poster/small/tt1727587/img"
    }
  ]
}
```
