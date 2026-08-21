// Adapters for migrating existing chi / gin / net-http code onto zip
// without rewriting handlers. These are MIGRATION TOOLS — each adapter
// costs ~5% perf vs native Fiber dispatch and exists so a service can
// roll onto zip incrementally. Replace adapted routes with native
// zip handlers when feasible.

package zip

import (
	"net/http"

	"github.com/gofiber/fiber/v3/middleware/adaptor"
)

// AdaptNetHTTP wraps an http.Handler so it can be mounted on a zip
// router. Use for stdlib code:
//
//	app.Mount("/legacy/net", zip.AdaptNetHTTP(httpHandler))
//
// Migration tool — costs ~5% perf vs native Fiber. Replace with native
// zip handlers when feasible.
func AdaptNetHTTP(h http.Handler) Handler {
	// Streams — see adapt_stream.go. There is ONE adapter rather than a
	// buffering one and a streaming one, because "which adapter did this route
	// get" is exactly the question nobody should have to ask to explain why
	// server-sent events 500 on one mount and work on another.
	return adaptStreaming(h)
}

// AdaptNetHTTPFunc wraps an http.HandlerFunc.
//
// Migration tool — costs ~5% perf vs native Fiber.
func AdaptNetHTTPFunc(h http.HandlerFunc) Handler {
	return adaptStreaming(h)
}

// AdaptNetHTTPMiddleware wraps a stdlib middleware (func(http.Handler) http.Handler).
//
// Migration tool — costs ~5% perf vs native Fiber.
func AdaptNetHTTPMiddleware(mw func(http.Handler) http.Handler) Handler {
	wrapped := adaptor.HTTPMiddleware(mw)
	return func(c *Ctx) error { return wrapped(c.fc) }
}

// Mount registers an http.Handler at the given prefix. Implicit
// chi.Router and gin.Engine support flows through AdaptNetHTTP via
// their respective ServeHTTP methods — both are http.Handlers.
//
//	app.Mount("/legacy/chi",  zip.AdaptNetHTTP(chiRouter))
//	app.Mount("/legacy/gin",  zip.AdaptNetHTTP(ginEngine))
//	app.Mount("/legacy/iam",  zip.AdaptNetHTTP(beegoApp.HandlerWrapper()))
//
// Migration tool — costs ~5% perf vs native Fiber.
func (a *App) Mount(prefix string, h http.Handler) {
	a.All(prefix+"/*", AdaptNetHTTP(h))
}
