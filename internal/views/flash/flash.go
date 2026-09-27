// Package flash carries one-shot user messages from a handler to the next
// rendered page. Handlers store the message in the session; the flash
// middleware moves it into the request context, and the layout renders it.
package flash

import "context"

// Flash is a single message with a display tone: "success", "warning" or
// "error" (anything else renders as neutral info).
type Flash struct {
	Message string
	Tone    string
}

type ctxKey struct{}

// With returns a context carrying f.
func With(ctx context.Context, f Flash) context.Context {
	return context.WithValue(ctx, ctxKey{}, f)
}

// Get returns the flash in ctx, or the zero value.
func Get(ctx context.Context) Flash {
	if f, ok := ctx.Value(ctxKey{}).(Flash); ok {
		return f
	}
	return Flash{}
}
