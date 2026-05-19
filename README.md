# zip

Hanzo's canonical Go web framework. Built on **Fiber v3 / fasthttp**.
Sinatra-style API. ZAP-typed handlers. Multi-language extension support
via [HIP-0105](https://github.com/hanzoai/hips).

**ONE framework. ZERO escape hatches. zip IS fast.**

```go
package main

import (
    "github.com/hanzoai/zip"
    "github.com/hanzoai/zip/middleware"
)

func main() {
    app := zip.New(zip.Config{})
    app.Use(middleware.Recover(), middleware.RequestID())

    app.Get("/health", func(c *zip.Ctx) error {
        return c.JSON(200, map[string]string{"status": "ok"})
    })

    v1 := app.Group("/v1")
    v1.Get("/users/:id", func(c *zip.Ctx) error {
        return c.JSON(200, map[string]string{
            "id":  c.Param("id"),
            "org": c.Org(),       // HIP-0026 gateway-minted X-Org-Id
            "user": c.User(),     // X-User-Id
        })
    })

    _ = app.Listen(":8080")
}
```

## Features

- **Sinatra/Express idiom** — `app.Get(path, fn)` is the primary API.
- **Typed handlers** — `zip.Get[In, Out](app, path, fn)` generates
  OpenAPI 3.1 spec and serves Swagger UI at `/docs`.
- **Hanzo identity built-in** — `c.Org() / c.User() / c.UserEmail()`
  pull JWT-validated values from gateway X-* headers per
  [HIP-0026](https://github.com/hanzoai/hips).
- **luxfi/log** for all logging — no `slog` or `zap` in zip.
- **Extension routes** — `app.Module("POST /v1/eval", "wasm", "./policy")`
  mounts any [HIP-0105](https://github.com/hanzoai/hips) extension as a
  route. Supports wasm (wazero) / goja / pyvm / starlark / v8go / native.
- **WebSocket** — `wsx.Upgrade(fn)` via fasthttp/websocket.
- **SSE / streaming** — `c.SendStreamWriter` (Fiber v3 native).
- **Drop-in migration** — `app.Mount("/legacy", chiRouter)` for any
  `http.Handler` (chi, gin, beego, net/http).
- **ZAP RPC** — `app.ZAPRegistry()` and `app.ZAPListen()` for binary
  RPC on a separate port. Dispatch surface stable; wire integration in
  follow-up PR.

## Install

```bash
go get github.com/hanzoai/zip
```

Module path: `github.com/hanzoai/zip`. Go version: 1.26.3 (forced by
luxfi/log).

## Architecture

- `zip.App` wraps `*fiber.App`. One binary, one server, no escape
  hatches — no `.Fast` variant API, no second router.
- `zip.Ctx` wraps `fiber.Ctx` and adds Hanzo identity sugar
  (`c.Org() / c.User() / c.IsAdmin() / c.RequestID() / c.Log()`).
- Handlers are `func(c *zip.Ctx) error`. Returning a `*zip.HTTPError`
  controls the response status; everything else becomes 500 JSON.
- Middleware lives in `zip/middleware/` — `Recover`, `Logger`,
  `RequestID`, `Auth`, `RateLimit`, `CORS`, `MaxBody`, `Telemetry`,
  `StripIdentityHeaders`.
- Adapters in `zip/adapt.go` — `AdaptNetHTTP / AdaptNetHTTPFunc /
  AdaptNetHTTPMiddleware`. Migration tools only — replace adapted
  routes with native handlers when feasible.
- Extension runtime contract in `zip/runtime/` — duck-typed
  `runtime.Loader` interface; zip does NOT pull `hanzoai/base` as a
  dep. Service binaries construct `*extruntime.Loader` and inject via
  `zip.Config.Loader`.

## Examples

| Example | Demonstrates |
|---|---|
| `examples/hello` | Minimal Sinatra-style API |
| `examples/zap-typed` | Generic typed handler + auto-OpenAPI |
| `examples/subsystem-mount` | HIP-0106 `Mount(*App, deps)` idiom |
| `examples/module-routes` | `app.Module()` over a runtime.Loader |
| `examples/websocket` | `wsx.Upgrade` echo server |
| `examples/sse-streaming` | Server-Sent Events via SendStreamWriter |
| `examples/migrate-from-gin` | gin→zip mechanical port |
| `examples/migrate-from-chi` | chi→zip via AdaptNetHTTP |
| `examples/migrate-from-beego` | beego→zip via AdaptNetHTTP |

## Migration

| From | Adapter |
|---|---|
| net/http | `zip.AdaptNetHTTP(httpHandler)` |
| chi | `zip.AdaptNetHTTP(chiRouter)` (chi.Router IS http.Handler) |
| gin | `zip.AdaptNetHTTP(ginEngine)` (gin.Engine IS http.Handler) |
| beego | `app.Mount("/legacy/iam", beeApp.Handlers)` |

Each adapter is doc'd "MIGRATION TOOL — costs ~5% perf vs native Fiber
dispatch. Replace with native handlers when feasible."

See `docs/MIGRATION.md` for per-framework recipes.

## License

Apache-2.0 (carry-forward from upstream zeekay/zip license metadata).
