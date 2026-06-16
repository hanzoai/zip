package main

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestExpressInZip is the proof point: the legacy TS handler in app.ts,
// transpiled by real esbuild, loaded into real goja, mounted on real
// Fiber via zip, exercised over a real HTTP roundtrip.
func TestExpressInZip(t *testing.T) {
	app, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("GET /legacy/foo", func(t *testing.T) {
		resp, err := app.Fiber().Test(httptest.NewRequest("GET", "/legacy/foo", nil))
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		assertContainsAll(t, string(body), `"ok":true`, `"path":"/foo"`, `"body":null`)
	})

	t.Run("POST /legacy/bar echoes body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/legacy/bar", strings.NewReader(`{"x":1}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Fiber().Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		assertContainsAll(t, string(body), `"ok":true`, `"path":"/bar"`, `"x":1`)
	})
}

func assertContainsAll(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Fatalf("body %q missing %q", body, w)
		}
	}
}
