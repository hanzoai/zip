# Middleware reference

All zip middleware is a `zip.Handler` (not raw `fiber.Handler`) so the
user-facing handler signature stays uniform. Install via `app.Use(...)`.

```go
import "github.com/hanzoai/zip/middleware"

app.Use(
    middleware.Recover(),
    middleware.RequestID(),
    middleware.Logger(app.Logger()),
)
```

## Recover

Catches handler panics, logs the stack via luxfi/log, and returns a 500
JSON response. Always include first.

```go
app.Use(middleware.Recover())
```

## RequestID

Injects `X-Request-Id`. If the incoming request carries one, it's
preserved; otherwise zip mints a 16-byte hex value. Available via
`c.RequestID()`.

```go
app.Use(middleware.RequestID())
```

## Logger

Wraps each request with a per-request luxfi/log child logger that
includes `request_id`, `org`, `user`. Logs the request line at
completion with `status` + `dur_ms`.

```go
app.Use(middleware.Logger(app.Logger()))
```

## Timeout

Per-request context deadline. Best-effort — Fiber's fasthttp-backed Ctx
does not natively propagate context cancellation through the request
lifetime; downstream code that consumes `c.Context()` (DB calls, HTTP
clients) gets the deadline.

```go
app.Use(middleware.Timeout(30 * time.Second))
```

## MaxBody

Rejects request bodies larger than `n` bytes with 413.

```go
app.Use(middleware.MaxBody(1 << 20))  // 1 MiB
```

## CORS

```go
app.Use(middleware.CORS(middleware.CORSConfig{
    AllowOrigins:  []string{"https://app.hanzo.ai"},
    AllowMethods:  []string{"GET", "POST"},
    AllowHeaders:  []string{"Content-Type", "Authorization"},
    AllowCreds:    true,
    MaxAge:        86400,
}))
```

## Auth

When the service is fronted by hanzoai/gateway, the gateway mints the
`X-Org-Id / X-User-Id / X-User-Email` headers from validated JWTs. Auth
middleware trusts those headers on the gateway-fronted path; on direct
deployments it falls back to verifying `Authorization: Bearer <token>`
via an injected `AuthVerifier`.

```go
// Trust gateway only:
app.Use(middleware.Auth(nil))

// Verify bearer tokens in-process:
app.Use(middleware.Auth(myIAMVerifier))
```

## StripIdentityHeaders

For deployments that do NOT run behind hanzoai/gateway — strips
client-supplied `X-Org-Id / X-User-Id / X-User-Email / X-User-IsAdmin /
X-Roles / X-User-Permissions` before any other middleware. Per
HIP-0026, only gateway-minted identity is trusted.

```go
app.Use(middleware.StripIdentityHeaders(), middleware.Auth(myVerifier))
```

## RateLimit

Per-org (or per-IP fallback) in-memory token bucket. Single-pod only —
multi-pod deployments must rate-limit at the gateway.

```go
app.Use(middleware.RateLimit(middleware.RateLimitConfig{
    Limit:  100,
    Window: time.Minute,
}))
```

## Telemetry

Plumbs request metrics to any `O11ySink` implementation. nil sink is a
no-op.

```go
app.Use(middleware.Telemetry(myO11ySink))
```

## Order

Recommended order for a Hanzo service:

```go
app.Use(
    middleware.Recover(),              // 1. always first
    middleware.StripIdentityHeaders(), // 2. only if not behind gateway
    middleware.RequestID(),            // 3. mint request id
    middleware.Logger(app.Logger()),   // 4. log with request id
    middleware.Telemetry(o11y),        // 5. metrics
    middleware.Auth(verifier),         // 6. validate identity
    middleware.RateLimit(rlCfg),       // 7. limit authenticated requests
    middleware.CORS(corsCfg),          // 8. last — close to response
)
```
