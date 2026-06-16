// esbuild.go wraps esbuild's pure-Go API (no CGO) so zip can compile
// TS / modern-JS handler source down to ES5 that the embedded goja VM
// executes. This is the build step of zip's TS migration path:
//
//	TS source --esbuild target=es5--> ES5 JS --drop into--> embedded goja
//
// Run TranspileToES5 at service startup (or at build time) to compile
// bundled handlers before serving; the resulting bytes are handed to
// JSRuntime.LoadModule / Eval.
package runtime

import (
	"fmt"

	"github.com/evanw/esbuild/pkg/api"
)

// ESOptions configures a transpile. Zero value is a sane default:
// TypeScript loader, ES5 target, no minification, CommonJS format so
// `module.exports = ...` survives into the goja-loadable output.
type ESOptions struct {
	// Loader selects how the source is parsed. "ts", "tsx", "jsx", or
	// "js". Empty defaults to "ts" (TS is a superset of JS, so plain JS
	// also parses).
	Loader string

	// Minify enables identifier/whitespace minification.
	Minify bool

	// Sourcefile is the logical filename used in error messages.
	Sourcefile string
}

// TranspileToES5 compiles src (TS or modern JS) to ES5 JavaScript ready
// for goja. The output is CommonJS-format so a module that does
// `module.exports = handler` can be loaded via JSRuntime.LoadModule and
// resolved with require(). Returns a non-nil error if esbuild reports
// any error-level diagnostic.
func TranspileToES5(src []byte, opts ESOptions) ([]byte, error) {
	loader := loaderFor(opts.Loader)
	sourcefile := opts.Sourcefile
	if sourcefile == "" {
		sourcefile = "handler" + extFor(loader)
	}

	result := api.Transform(string(src), api.TransformOptions{
		Loader:            loader,
		Target:            api.ES2015, // ES2015 is the lowest target esbuild emits; goja runs it.
		Format:            api.FormatCommonJS,
		Sourcefile:        sourcefile,
		MinifyWhitespace:  opts.Minify,
		MinifyIdentifiers: opts.Minify,
		MinifySyntax:      opts.Minify,
	})
	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("zip/runtime: esbuild: %s", formatMessages(result.Errors))
	}
	return result.Code, nil
}

func loaderFor(name string) api.Loader {
	switch name {
	case "", "ts":
		return api.LoaderTS
	case "tsx":
		return api.LoaderTSX
	case "jsx":
		return api.LoaderJSX
	case "js":
		return api.LoaderJS
	default:
		return api.LoaderTS
	}
}

func extFor(l api.Loader) string {
	switch l {
	case api.LoaderTSX:
		return ".tsx"
	case api.LoaderJSX, api.LoaderJS:
		return ".js"
	default:
		return ".ts"
	}
}

func formatMessages(msgs []api.Message) string {
	out := ""
	for i, m := range msgs {
		if i > 0 {
			out += "; "
		}
		out += m.Text
		if m.Location != nil {
			out += fmt.Sprintf(" (%s:%d:%d)", m.Location.File, m.Location.Line, m.Location.Column)
		}
	}
	return out
}
