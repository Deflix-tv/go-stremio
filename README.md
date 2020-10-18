# go-stremio

Stremio addon SDK for Go

## Contents

1. [Introduction](#introduction)
2. [About this SDK](#about-this-sdk)
3. [Features](#features)
4. [Example](#example)
5. [Advantages](#advantages)
6. [Related projects](#related-projects)

## Introduction

[Stremio](https://www.stremio.com/) is a modern media center that's a one-stop solution for your video entertainment. You discover, watch and organize video content from easy to install addons.

These addons run remotely as a web service, so they can't do any harm to your computer. This is different from how addons work for Kodi for example, where they run locally on your computer and have dozens of third party dependencies. There have been several security incidents with Kodi addons like [this one](https://www.reddit.com/r/Addons4Kodi/comments/axhmcw/public_service_announcement_remove_civitas_repo/) and even the Kodi developers themselves [warn of the dangers of third party Kodi addons](https://kodi.tv/article/warning-be-aware-what-additional-add-ons-you-install).

## About this SDK

When developing a Stremio addon, you're essentially developing a web service. But there are some defined routes, expected behavior, JSON structure etc., so instead of having to figure all of this out on your own before you've got even a basic addon running, using an SDK can get you up to speed much faster, as it takes care of all of this automatically.

But the [official Stremio addon SDK](https://github.com/Stremio/stremio-addon-sdk) is for Node.js only.

This SDK is for Go!

It provides the most important parts of the Node.js SDK and depending on the requirements of you, the libary users, it will be extended to provide more in the future.

## Features

- [x] All required *types* for building catalog and stream addons
- [x] Web server with graceful shutdown
  - [x] With optional channel to be notified about a shutdown
- [x] CORS middleware to allow requests from Stremio
- [x] Health check endpoint
- [x] Optional profiling endpoints (for `go pprof`)
- [x] Optional request logging
  - [x] With optional movie / TV show name in the log (instead of just the IMDb ID)
  - [x] With optional client IP address and user agent logging to create privacy-preserving addons
- [x] Optional cache control and ETag handling
- [x] Optional custom middlewares
- [x] Optional custom endpoints
- [x] Custom user data (users can have *settings* for your addon!)
  - [x] Including the handling of Stremio's requests to the "/configure" endpoint to show a webpage for the addon's configuration
  - [x] With optional URL-safe Base64 decoding and JSON unmarshalling
- [x] Addon installation callback (manifest endpoint)
- [x] Cinemeta client in the independent `cinemeta` package
- [x] Optional stream ID filtering via regex

Current *non*-features, as they're usually part of a reverse proxy deployed in front of the service:

- TLS termination (for using HTTP*S*)
- Rate limiting (against DoS attacks)
- Compression (like gzip)

## Example

Full examples can be found in [examples](./examples). Here's a part of the one for a stream addon:

```go
package main

import (
    "context"

    "github.com/deflix-tv/go-stremio"
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
    // Let the movieHandler handle the "movie" type
    streamHandlers := map[string]stremio.StreamHandler{"movie": movieHandler}

    addon, err := stremio.NewAddon(manifest, nil, streamHandlers, stremio.DefaultOptions)
    if err != nil {
        panic(err)
    }

    addon.Run(nil)
}

func movieHandler(ctx context.Context, id string, userData interface{}) ([]stremio.StreamItem, error) {
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
---------|-------------|--------
Direct SDK dependencies|9|4
Transitive SDK dependencies|90¹|35²
Size of a runnable addon|27 MB³|11-15 MB⁴
Number of artifacts to deploy|depends⁵|1
Runtime dependencies|Node.js|-
Concurrency|Single-threaded|Multi-threaded

¹) `ls -l node_modules | wc -l` - 1  
²) `go list -m all | wc -l` - 1 - (number of direct dependencies)  
³) `du -h --max-depth=0 node_modules`  
⁴) The smaller binary is easily achieved by compiling with `-ldflags "-s -w"`  
⁵) All your JavaScript files and the `package.json` if you can install the depencencies with `npm` on the server, otherwise (like in a Docker container) you also need all the `node_modules`, which are hundreds to thousands of files.  

Looking at the performance it depends a lot on what your addon does. Due to the single-threaded nature of Node.js, the more CPU-bound tasks your addon does, the bigger the performance difference will be (in favor of Go). Here we compare the simplest possible addon to be able to compare just the SDKs and not any additional overhead (like DB access):

On a [DigitalOcean](https://www.digitalocean.com/) "Droplet" of type "Basic" (shared CPU) with 2 cores and 2 GB RAM, which costs $15/month:

Criterium|Node.js addon|Go addon
---------|-------------|--------
Startup time to 1st request¹|400ms-4s|20-30ms
Max rps² @ 1000 connections|Local³: 1,000<br>Remote⁴: 1,000|Local³: 17,000<br>Remote⁴: 29,000
Memory usage @ 1000 connections|Idle: 42 MB<br>Load⁵: 73 MB|Idle: 11 MB<br>Load⁵: 45 MB

On a [DigitalOcean](https://www.digitalocean.com/) "Droplet" of type "CPU-Optimized" (dedicated CPU) with 2 cores and 4 GB RAM, which costs $40/month:

Criterium|Node.js addon|Go addon
---------|-------------|--------
Startup time to 1st request¹|200-400ms|9-20ms
Max rps² @ 1000 connections|Local³: 5,000<br>Remote⁴: 1,000|Local³: 39,000<br>Remote⁴: 39,000
Memory usage @ 1000 connections|Idle: 42 MB<br>Load⁵: 90 MB|Idle: 11 MB<br>Load⁵: 47 MB

¹) Measured using [ttfok](https://github.com/doingodswork/ttfok) and the code in [benchmark](benchmark). This metric is relevant in case you want to use a "serverless functions" service (like [AWS Lambda](https://aws.amazon.com/lambda/) or [Vercel](https://vercel.com/) (former ZEIT Now)) that doesn't keep your service running between requests.  
²) Max number of requests per second where the p99 latency is still < 100ms  
³) The load testing tool ran on a different server, but in the same datacenter and the requests were sent within a private network  
⁴) The load testing tool ran on a different server *in a different datacenter of another cloud provider in another city* for more real world-like circumstances  
⁵) Resident size (`RES` in `htop`) at a request rate *half* of what we measured as maximum  

The load tests were run under the following circumstances:

- We used the addon code, load testing tool and setup described in [benchmark](benchmark)
- We ran the Node.js and Go service on the same Droplet and conducted the benchmark on the same day, several minutes apart, so that the resource sharing of the VPS is about the same. Note that when you try to reproduce the benchmark results, a different VPS could be subject to more or less resource sharing with other VPS on the virtualization host. Other times of day can also lead to differing benchmark results (e.g. low traffic on a Monday morning, high traffic on a Saturday evening).
- The load tests ran for 60s (to have a somewhat meaningful p99 value), with previous warmup
- The client servers (both the one in the same DC and the one in a different DC of another cloud provider in another city) used to run the load testing tool were high-powered (8 *dedicated* cores, 32 GB RAM)

Additional observations:

- The Go service's response times were generally lower across all request rates
- The Go service's response times had a much lower deviation, i.e. they were more stable. With less than 60s of time for the load test the Node.js service fared even worse, because outliers lead to a higher p99 latency.
- We also tested on a lower-powered server by a cheap cloud provider (also 2 core, 2 GB RAM, but the CPU was generally worse). In this case the difference between the Node.js and the Go service was even higher. The Go service is perfectly fitted for scaling out with multiple cheap servers.
- We also tested with different amounts of connections. With more connections the difference between the Node.js and the Go service was also higher. In a production deployment you want to be able to serve as many users as possible, so this goes in favor of the Go service as well.

> Note:
>
> - This Go SDK is still young. Some features will be added in the future that might decrease its performance, while others will increase it.  
> - The Node.js addon was run as a single instance. You can do more complex deployments with a load balancer like [HAProxy](https://www.haproxy.org/) and multiple instances of the same Node.js service on a single machine to take advantage of multiple CPU cores. But then you should also activate preforking in the Go addon for using several OS processes in parallel, which we didn't do.  

## Related projects

- [The official Stremio addon SDK for Node.js](https://github.com/Stremio/stremio-addon-sdk)
- [Stremio addon SDK for Rust](https://github.com/sleeyax/stremio-addon-sdk)
