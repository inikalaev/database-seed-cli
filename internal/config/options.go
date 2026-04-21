package config

import (
	"fmt"
	"os"
)

// Options controls the behaviour of Load/Unmarshal/Save/Marshal calls that
// need to surface non-fatal information (e.g., version-absent warnings).
//
// The zero value is safe: a nil Logger falls back to stderr so existing CLI
// callers keep the same visible behaviour. Library consumers that want the
// messages programmatically can supply a callback via WithLogger.
type Options struct {
	// Logger receives human-readable warning messages. nil = write to stderr.
	Logger func(message string)
}

// Option tunes an Options value. Passed as the variadic tail of Load /
// Unmarshal / Save / Marshal so the call sites that don't care stay short.
type Option func(*Options)

// WithLogger directs warnings to fn instead of stderr. fn is called
// synchronously on the Load/Unmarshal goroutine; don't block in it.
func WithLogger(fn func(string)) Option {
	return func(o *Options) { o.Logger = fn }
}

func resolveOptions(opts []Option) Options {
	var o Options
	for _, f := range opts {
		if f != nil {
			f(&o)
		}
	}
	return o
}

// warn emits msg via o.Logger if set, otherwise to stderr with a "seed: "
// prefix. Never returns an error — warnings must not fail the caller.
func (o Options) warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if o.Logger != nil {
		o.Logger(msg)
		return
	}
	fmt.Fprintln(os.Stderr, "seed: "+msg)
}
