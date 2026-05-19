# zip

Canonical Hanzo Go web framework. Sinatra-style API built on Fiber v3 / fasthttp. The ONE Go web framework — no `.Fast` escape hatch.

[![Status](https://img.shields.io/badge/status-beta-blue)]()
[![License](https://img.shields.io/badge/license-MIT-blue)]()

## Quick start

```bash
go get github.com/hanzoai/zip
```

```go
package main

import "github.com/hanzoai/zip"

func main() {
    app := zip.New()
    app.Get("/", func(c zip.Ctx) error {
        return c.SendString("hello world!")
    })
    app.Listen(":1337")
}
```

## What this is

`zip` is the canonical web framework every Hanzo Go service uses (`iam`, `kms`, `gateway`, `commerce`, `base`, `cloud`, ...). Built on Fiber v3 over `fasthttp`. Sinatra/Express-style primary API for fast prototyping, fasthttp-grade throughput in production.

## Specs

Implements:
- HIP-0106 Unified Cloud Binary (web framework section)

Used by every subsystem listed in HIP-0106.

## Architecture

```
   your handler  ->  zip.App  ->  Fiber v3  ->  fasthttp  ->  net.Listener
                       |
                       +--  middlewares: identity strip/mint, JWT, CORS, rate-limit
                       +--  mount points for cloud subsystems (Mount(app, deps))
```

Zero parallel frameworks. No alternate `Fast` API. Same handler shape works in standalone services and inside the unified `hanzoai/cloud` binary.


---

# zip
A zippy web micro-framework. Just a tiny bit of sugar for Go's excellent
`net/http` package. Inspired by [bottle][bottle], [express][express], [web.go][web.go], etc.

## Install
Zip can be installed with `go get`:

```bash
$ go get zeekay.io/zip
```

## Usage
A simple hello world web app looks like this:

```go
package main

import "zeekay.io/zip"

func main() {
    zip.Get("/", func(req zip.Req, res zip.Res) {
        res.End("hello world!")
    })

    // Run and listen on port 1337.
    zip.Run(":1337")
}
```

You can run the server with `go run`:

```bash
$ go run hello.go
```

## Examples
There are a few examples in [examples/][examples] for you to play with:

- [hello.go][hello.go]
- [json.go][json.go]
- [websocket.go][websocket.go]

[examples]:     https://github.com/zeekay/zip/blob/master/examples
[hello.go]:     https://github.com/zeekay/zip/blob/master/examples/hello/hello.go
[json.go]:      https://github.com/zeekay/zip/blob/master/examples/json/json.go
[websocket.go]: https://github.com/zeekay/zip/blob/master/examples/websocket/websocket.go
[bottle]:       http://bottlepy.org
[express]:      http://expressjs.com
[web.go]:       http://webgo.io
