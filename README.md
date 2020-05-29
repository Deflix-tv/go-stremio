# Stremio addon SDK

A Go library for creating Stremio addons

## Contents

1. Introduction
2. About this SDK
3. Example
4. Related projects

## Introduction

[Stremio](https://www.stremio.com/) is a modern media center that's a one-stop solution for your video entertainment. You discover, watch and organize video content from easy to install addons.

These addons run remotely as a web service, so they can't do any harm to your computer. This is different from how addons work for Kodi for example, where they run locally on your computer and have dozens of third party dependencies. There have been several security incidents with Kodi addons like [this one](https://www.reddit.com/r/Addons4Kodi/comments/axhmcw/public_service_announcement_remove_civitas_repo/) and even the Kodi developers themselves [warn of the dangers of third party Kodi addons](https://kodi.tv/article/warning-be-aware-what-additional-add-ons-you-install).

## About this SDK

When developing a Stremio addon, you're essentially developing a web service. But there are some defined routes, expected behavior, JSON structure etc., so instead of having to figure all of this out on your own before you've got even a basic addon running, using an SDK can get you up to speed much faster, as it takes care of all of this automatically.

But the [official Stremio addon SDK](https://github.com/Stremio/stremio-addon-sdk) is for Node.js only.

This SDK is for Go!

It provides the most important parts of the Node.js SDK and depending on the requirements of you, the libary users, it will be extended to provide more in the future.

## Example

Full examples can be found in [examples](./examples). Here's a part of the one for a stream addon:

```go
package main

import (
    "log"

    "github.com/deflix-tv/stremio-addon-sdk"
)

var (
    manifest = stremio.Manifest{
        ID:          "com.example.blender-streams",
        Name:        "Blender movie streams",
        Description: "Stream addon for free movies that were made with Blender",
        // ...
    }
)

func main() {
    streamHandlers := map[string]stremio.StreamHandler{"movie": streamHandler}

    addon, err := stremio.NewAddon(manifest, nil, streamHandlers, stremio.Options{Port: 8080})
    if err != nil {
        log.Fatalf("Couldn't create addon: %v", err)
    }

    addon.Run()
}

func streamHandler(id string) ([]stremio.StreamItem, error) {
    // We only serve Big Buck Bunny and Sintel
    if id == "tt1254207" {
        return []stremio.StreamItem{
            // Torrent stream
            {
                InfoHash: "dd8255ecdc7ca55fb0bbf81323d87062db1f6d1c",
                // Stremio recommends to set the quality as title, as the streams
                // are shown for a specific movie so the user knows the title.
                Title:     "1080p (torrent)",
                FileIndex: 1,
            },
            // HTTP stream
            {
                URL:   "http://distribution.bbb3d.renderfarming.net/video/mp4/bbb_sunflower_1080p_30fps_normal.mp4",
                Title: "1080p (HTTP stream)",
            },
        }, nil
    } else if id == "tt1727587" {
        // ...
    }
    return nil, stremio.NotFound
}
```

## Related projects

- [The official Node.js SDK](https://github.com/Stremio/stremio-addon-sdk)
- [Stremio addon SDK for Rust](https://github.com/sleeyax/stremio-addon-sdk)
