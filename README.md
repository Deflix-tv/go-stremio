# Stremio addon SDK

A Go library for creating Stremio addons

## Contents

1. [Introduction](#introduction)
2. [About this SDK](#about-this-sdk)
3. [Example](#example)
4. [Advantages](#advantages)
5. [Related projects](#related-projects)

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

## Advantages

Some reasons why you might want to consider developing an addon in Go with this SDK:

Criterium|Node.js addon|Go addon
---------|--------|-------------
Direct SDK dependencies|9|3
Transitive SDK dependencies|~100|1
Size of a deployable addon|20 MB|8 MB
Number of artifacts to deploy|depends|1
Runtime dependencies|Node.js|-
Concurrency|Single-threaded|Multi-threaded

Looking at the performance it depends a lot on what your addon does. Due to the single-threaded nature of Node.js, the more CPU-bound tasks your addon does, the bigger the performance difference will be (in favor of Go). Here we compare the simplest possible addon to be able to compare just the SDKs and not any additional overhead (like DB access):

Criterium|Node.js addon|Go addon
---------|--------|-------------
Startup time to 1st request¹|920-1300ms|5-20ms
Max rps² @ 100 users|5000|5000
Max rps² @ 1000 users<br>(2 core, 2 GB RAM, 5 €/mo)|4000|14000
Max rps² @ 1000 users<br>(2 core, 2 GB RAM, 10 €/mo)|7000|21000
Max rps² @ 1000 users<br>(8 dedicated cores, 32 GB RAM, 100 €/mo)|17000|43000
Memory usage @ 100 users³|70-80 MB|10-20 MB
Memory usage @ 1000 users³|90-100 MB|30-40 MB

¹) Measured using [ttfok](https://github.com/doingodswork/ttfok) and the code in [benchmark](benchmark)  
²) Max number of requests per second where the p99 latency is still < 100ms  
³) At a request rate *half* of what we measured as maximum

The load tests were run under the following cirumstances:

- We used the addon code, load testing tool and setup described in [benchmark](benchmark)
- The Load testing tool ran on a different server *in a different datacenter in another country* for more real world-like circumstances
- The load tests ran for 15s, with previous warmup

> Note:
>
> - This Go SDK is at its very beginning. Some features will be added in the future that might decrease its performance, while others will increase it.
> - The Node.js addon was run as a single instance. You can do more complex deployments with a load balancer like [HAProxy](https://www.haproxy.org/) and multiple instances of the same Node.js service on a single machine to take advantage of multiple CPU cores.

## Related projects

- [The official Node.js SDK](https://github.com/Stremio/stremio-addon-sdk)
- [Stremio addon SDK for Rust](https://github.com/sleeyax/stremio-addon-sdk)
