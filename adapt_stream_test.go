package zip

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

// serve starts the app on a real socket. A real socket is the point: fiber's
// in-memory Test helper collects the whole response, so it cannot tell a stream
// apart from a buffer, which is the exact distinction these tests exist to make.
func serve(t *testing.T, app *App) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = app.fiber.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}) }()
	t.Cleanup(func() { _ = app.fiber.Shutdown() })
	for i := 0; i < 100; i++ { // wait for accept
		c, err := net.DialTimeout("tcp", ln.Addr().String(), 50*time.Millisecond)
		if err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	return "http://" + ln.Addr().String()
}

func newApp() *App { return New(Config{DisableStartupMessage: true}) }

// THE ASSERTION THAT WAS FAILING. Every streaming handler in a mounted stdlib
// server begins by asking whether it may flush; the buffering adaptor answered
// no, and the handler returned 500 instead of a stream.
func TestAdaptNetHTTP_WriterIsAFlusher(t *testing.T) {
	app := newApp()
	app.All("/x/*", AdaptNetHTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := w.(http.Flusher); !ok {
			http.Error(w, "writer does not implement http.Flusher", http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, "flushable")
	})))
	res, err := http.Get(serve(t, app) + "/x/y")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || string(body) != "flushable" {
		t.Fatalf("status %d body %q — the mounted handler could not flush", res.StatusCode, body)
	}
}

// And it must actually STREAM: a frame written before the handler returns has to
// be readable before it returns, or "streaming" is buffering that compiles.
func TestAdaptNetHTTP_FramesArriveBeforeTheHandlerReturns(t *testing.T) {
	release := make(chan struct{})
	app := newApp()
	app.All("/sse/*", AdaptNetHTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		<-release // the handler is still running
		fmt.Fprint(w, "data: second\n\n")
	})))
	res, err := http.Get(serve(t, app) + "/sse/x")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("content-type %q — headers must be set before the body streams", ct)
	}
	done := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(res.Body).ReadString('\n')
		done <- line
	}()
	select {
	case line := <-done:
		if !strings.Contains(line, "first") {
			t.Fatalf("first frame %q", line)
		}
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("nothing arrived while the handler was still running — still buffering")
	}
	close(release)
}

// The ordinary case must not regress.
func TestAdaptNetHTTP_StatusAndHeadersSurvive(t *testing.T) {
	app := newApp()
	app.All("/j/*", AdaptNetHTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Trace", "abc")
		w.WriteHeader(http.StatusTeapot)
		fmt.Fprint(w, `{"ok":1}`)
	})))
	res, err := http.Get(serve(t, app) + "/j/k")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusTeapot {
		t.Fatalf("status %d, want 418", res.StatusCode)
	}
	if res.Header.Get("X-Trace") != "abc" || !strings.Contains(res.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("headers lost: %v", res.Header)
	}
	if string(body) != `{"ok":1}` {
		t.Fatalf("body %q", body)
	}
}

// A handler that writes nothing still has to answer.
func TestAdaptNetHTTP_SilentHandlerStillAnswers(t *testing.T) {
	app := newApp()
	app.All("/q/*", AdaptNetHTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))
	res, err := http.Get(serve(t, app) + "/q/z")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
}

// A panicking handler must close the stream rather than strand the reader.
func TestAdaptNetHTTP_PanicClosesTheStream(t *testing.T) {
	app := newApp()
	app.All("/p/*", AdaptNetHTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "partial")
		panic("boom")
	})))
	res, err := http.Get(serve(t, app) + "/p/x")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	done := make(chan struct{})
	go func() { _, _ = io.ReadAll(res.Body); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("reader stranded after the handler panicked")
	}
}
