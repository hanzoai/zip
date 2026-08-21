package zip

import (
	"io"
	"net/http"
	"strconv"
	"sync"

	"github.com/gofiber/fiber/v3/middleware/adaptor"
)

// A MOUNTED HANDLER CAN STREAM.
//
// fiber's own adaptor hands the stdlib handler a ResponseWriter that collects
// the whole body and only then copies it out. Two consequences, and the second
// is the one that bites: nothing reaches the client until the handler returns,
// and the writer does NOT implement http.Flusher — so a handler that streams
// does not merely buffer, it FAILS, on its own `w.(http.Flusher)` assertion.
// That is what made every server-sent-event route under a mounted net/http
// server answer 500 "writer does not implement http.Flusher" while the
// non-streaming routes beside it were fine.
//
// So the handler runs against a pipe instead. Its writes cross to the response
// as it makes them, and the pipe is SYNCHRONOUS — a Write does not return until
// the reader has taken the bytes — which is what makes Flush honest here rather
// than a stub: by the time a handler calls it, the bytes are already on their
// way and there is nothing withheld to push.
//
// The status and headers are read from the handler's FIRST act (WriteHeader, an
// implicit 200 on first Write, or the handler simply returning) and applied
// before the body streams, because fasthttp writes them once and cannot revise
// them afterwards.
type streamWriter struct {
	hdr    http.Header
	status int
	pw     *io.PipeWriter
	once   sync.Once
	ready  chan struct{}
}

func (w *streamWriter) Header() http.Header { return w.hdr }

func (w *streamWriter) WriteHeader(code int) {
	w.once.Do(func() {
		w.status = code
		close(w.ready)
	})
}

func (w *streamWriter) Write(b []byte) (int, error) {
	w.WriteHeader(http.StatusOK) // net/http's implicit 200, same rule
	return w.pw.Write(b)
}

// Flush satisfies http.Flusher, which is the whole point of this type.
func (w *streamWriter) Flush() { w.WriteHeader(http.StatusOK) }

// adaptStreaming mounts an http.Handler so that it can stream.
func adaptStreaming(h http.Handler) Handler {
	return func(c *Ctx) error {
		req, err := adaptor.ConvertRequest(c.fc, false)
		if err != nil {
			return err
		}
		// The request outlives this call — the handler runs while the body is
		// still streaming — so it carries the connection's context rather than
		// the pooled fiber one, and a client that goes away cancels it.
		req = req.WithContext(c.fc.RequestCtx())

		pr, pw := io.Pipe()
		w := &streamWriter{hdr: make(http.Header), status: http.StatusOK, pw: pw, ready: make(chan struct{})}

		go func() {
			// A panicking handler must close the pipe, or the reader below
			// blocks forever holding the connection open.
			defer func() {
				if r := recover(); r != nil {
					_ = pw.CloseWithError(http.ErrAbortHandler)
				} else {
					_ = pw.Close()
				}
				w.WriteHeader(http.StatusOK) // a handler that wrote nothing still answered
			}()
			h.ServeHTTP(w, req)
		}()

		<-w.ready

		resp := c.fc.Response()
		resp.SetStatusCode(w.status)
		size := -1 // chunked unless the handler stated a length
		for k, vs := range w.hdr {
			if k == "Content-Length" {
				if n, err := strconv.Atoi(vs[0]); err == nil {
					size = n
				}
				continue // fasthttp writes this itself from the stream size
			}
			for _, v := range vs {
				resp.Header.Add(k, v)
			}
		}
		resp.SetBodyStream(pr, size)
		return nil
	}
}
