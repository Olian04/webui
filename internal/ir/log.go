package ir

import (
	"context"
	"log/slog"
)

// discard is the logger of an app that was given none: it says nothing.
var discard = slog.New(slog.DiscardHandler)

// Discard is the logger of an app that was given none.
func Discard() *slog.Logger { return discard }

type loggerKey struct{}

// WithLogger returns ctx carrying the logger of the app that is answering, for the code that has no other way to reach
// it: a column's Format, which is called with no more than the value.
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, l)
}

// LoggerFrom is the logger ctx carries, or one that says nothing when it carries none.
func LoggerFrom(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return discard
}
