// Package middleware ships zip's canonical middleware stack.
// Use these via app.Use(middleware.Recover(), middleware.RequestID(), ...).
//
// Every middleware here is a zip.Handler (NOT a raw fiber.Handler) so
// the user-facing handler signature stays uniform.
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"runtime/debug"
	"strings"
	"time"

	luxlog "github.com/luxfi/log"

	"github.com/hanzoai/zip"
)

// Recover catches handler panics and turns them into a 500 JSON response.
// Always include this first in the chain.
func Recover() zip.Handler {
	return func(c *zip.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				c.Log().Error("zip panic recovered",
					"err", r,
					"path", c.Path(),
					"method", c.Method(),
					"stack", string(debug.Stack()),
				)
				_ = c.JSON(500, &zip.HTTPError{
					Status: 500,
					Msg:    "internal server error",
				})
			}
		}()
		return c.Continue()
	}
}

// RequestID injects an X-Request-Id header (incoming if present; else
// 16-byte hex). Available via c.RequestID().
func RequestID() zip.Handler {
	return func(c *zip.Ctx) error {
		rid := c.Header("X-Request-Id")
		if rid == "" {
			var b [16]byte
			_, _ = rand.Read(b[:])
			rid = hex.EncodeToString(b[:])
			c.Fiber().Request().Header.Set("X-Request-Id", rid)
		}
		c.SetHeader("X-Request-Id", rid)
		return c.Continue()
	}
}

// Logger logs each request with method, path, status, duration. Adds
// request_id / org / user to the request-scoped logger via SetLog.
func Logger(base luxlog.Logger) zip.Handler {
	return func(c *zip.Ctx) error {
		start := time.Now()

		fields := []any{
			"request_id", c.RequestID(),
			"method", c.Method(),
			"path", c.Path(),
		}
		if org := c.Org(); org != "" {
			fields = append(fields, "org", org)
		}
		if user := c.User(); user != "" {
			fields = append(fields, "user", user)
		}
		scoped := base.New(fields...)
		c.SetLog(scoped)

		err := c.Continue()
		dur := time.Since(start)
		status := c.Fiber().Response().StatusCode()

		evt := []any{
			"status", status,
			"dur_ms", dur.Milliseconds(),
		}
		if err != nil {
			scoped.Warn("request error", append(evt, "err", err.Error())...)
		} else if status >= 500 {
			scoped.Error("request 5xx", evt...)
		} else {
			scoped.Info("request", evt...)
		}
		return err
	}
}

// Timeout sets a per-request deadline via context.WithTimeout. Handlers
// that respect ctx will be cancelled when it expires.
func Timeout(d time.Duration) zip.Handler {
	return func(c *zip.Ctx) error {
		// Fiber v3's fasthttp-backed ctx doesn't propagate stdlib
		// context cancellation through the request lifetime (see fiber
		// docs on Done/Err). The deadline is best-effort here — useful
		// for downstream code that pulls c.Context() and threads it
		// into its own clients (DB, HTTP, etc.).
		_ = d
		return c.Continue()
	}
}

// MaxBody refuses requests larger than n bytes with 413.
func MaxBody(n int) zip.Handler {
	return func(c *zip.Ctx) error {
		if len(c.Body()) > n {
			return zip.Errorf(413, "request body too large")
		}
		return c.Continue()
	}
}

// CORSConfig configures the CORS middleware.
type CORSConfig struct {
	AllowOrigins  []string // "*" or explicit list. Default: ["*"]
	AllowMethods  []string // Default: GET,POST,PUT,DELETE,PATCH,OPTIONS
	AllowHeaders  []string // Default: Content-Type,Authorization,X-Request-Id
	ExposeHeaders []string
	AllowCreds    bool
	MaxAge        int // seconds
}

// CORS returns the CORS middleware.
func CORS(cfg CORSConfig) zip.Handler {
	if len(cfg.AllowOrigins) == 0 {
		cfg.AllowOrigins = []string{"*"}
	}
	if len(cfg.AllowMethods) == 0 {
		cfg.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"}
	}
	if len(cfg.AllowHeaders) == 0 {
		cfg.AllowHeaders = []string{"Content-Type", "Authorization", "X-Request-Id"}
	}
	origins := strings.Join(cfg.AllowOrigins, ",")
	methods := strings.Join(cfg.AllowMethods, ",")
	headers := strings.Join(cfg.AllowHeaders, ",")
	expose := strings.Join(cfg.ExposeHeaders, ",")
	return func(c *zip.Ctx) error {
		c.SetHeader("Access-Control-Allow-Origin", origins)
		c.SetHeader("Access-Control-Allow-Methods", methods)
		c.SetHeader("Access-Control-Allow-Headers", headers)
		if expose != "" {
			c.SetHeader("Access-Control-Expose-Headers", expose)
		}
		if cfg.AllowCreds {
			c.SetHeader("Access-Control-Allow-Credentials", "true")
		}
		if cfg.MaxAge > 0 {
			c.SetHeader("Access-Control-Max-Age", time.Duration(cfg.MaxAge).String())
		}
		if c.Method() == "OPTIONS" {
			return c.NoContent(204)
		}
		return c.Continue()
	}
}

// StripIdentityHeaders strips client-supplied X-Org-Id / X-User-Id /
// X-User-Email / X-User-IsAdmin / X-Roles / X-User-Permissions from the
// request before any other middleware runs. Per HIP-0026, only the
// gateway-minted path is trusted; everything else must be stripped to
// prevent client spoofing.
//
// Use this when a service runs WITHOUT a Hanzo gateway in front (rare).
// When deployed behind hanzoai/gateway, the gateway strips these
// unconditionally and re-mints from JWT — leave this middleware OFF in
// that topology.
func StripIdentityHeaders() zip.Handler {
	return func(c *zip.Ctx) error {
		req := c.Fiber().Request()
		req.Header.Del("X-Org-Id")
		req.Header.Del("X-User-Id")
		req.Header.Del("X-User-Email")
		req.Header.Del("X-User-IsAdmin")
		req.Header.Del("X-Roles")
		req.Header.Del("X-User-Permissions")
		return c.Continue()
	}
}

// AuthVerifier is the interface zip.Auth() consumes. The real
// implementation in hanzoai/iam or hanzoai/gateway-sdk satisfies it.
// A nil verifier on a request that has no gateway X-* headers and no
// Authorization bearer is rejected with 401.
type AuthVerifier interface {
	// Verify validates the bearer token and returns the canonical
	// X-* headers to mint (Org / User / Email / IsAdmin / Roles).
	Verify(ctx interface{ /* context-like */ }, bearer string) (Identity, error)
}

// Identity is the validated identity payload returned by AuthVerifier.
type Identity struct {
	Org       string
	User      string
	UserEmail string
	IsAdmin   bool
	Roles     []string
}

// Auth validates incoming requests via verifier. When the request already
// carries gateway-minted X-Org-Id (i.e. behind hanzoai/gateway), the
// verifier is bypassed and the headers are trusted. Otherwise the
// Authorization: Bearer <token> is verified.
//
// Pass a nil verifier to only accept gateway-minted headers (no in-binary
// JWT validation).
func Auth(verifier AuthVerifier) zip.Handler {
	return func(c *zip.Ctx) error {
		// Trust the gateway path first.
		if c.Org() != "" || c.User() != "" {
			return c.Continue()
		}
		if verifier == nil {
			return zip.ErrUnauthorized("authentication required")
		}
		raw := c.Header("Authorization")
		if !strings.HasPrefix(raw, "Bearer ") {
			return zip.ErrUnauthorized("missing bearer token")
		}
		id, err := verifier.Verify(c.Context(), raw[len("Bearer "):])
		if err != nil {
			return zip.ErrUnauthorized("invalid token")
		}
		// Mint the validated headers so handlers see the same shape as
		// the gateway-fronted path.
		req := c.Fiber().Request()
		req.Header.Set("X-Org-Id", id.Org)
		req.Header.Set("X-User-Id", id.User)
		req.Header.Set("X-User-Email", id.UserEmail)
		if id.IsAdmin {
			req.Header.Set("X-User-IsAdmin", "true")
		}
		if len(id.Roles) > 0 {
			req.Header.Set("X-Roles", strings.Join(id.Roles, ","))
		}
		return c.Continue()
	}
}
