// Package ui is graft's output surface: the two streams, the error format, and the one
// colour decision.
//
// SPEC.md splits the streams by audience rather than by severity: stdout carries the content
// a caller asked for — output shaped for a program, and equally output a person asked to see,
// such as --version, help, and a listing — while progress, summaries, notes about the absence
// of content, and errors go to stderr, so that a pipe is never corrupted by text a human was
// meant to read. This package owns both, and every stream any
// other component is handed is one it wrapped: cobra renders its own help and usage, and a
// stream it was given raw would put bytes on stdout that nothing here can see fail.
package ui

import (
	"fmt"
	"io"
)

// UI is graft's output surface: the two streams and one colour decision.
type UI struct {
	out   *recorder
	err   *recorder
	color bool
}

// New builds a UI writing machine-readable output to out and everything else to err.
// color says whether styling is allowed; see [ColorEnabled].
func New(out, err io.Writer, color bool) *UI {
	return &UI{out: &recorder{w: out}, err: &recorder{w: err}, color: color}
}

// Out returns the machine-readable stream as a writer whose failures the UI records.
// It exists so a dependency that renders its own output writes through the UI rather than
// around it.
func (u *UI) Out() io.Writer { return u.out }

// Err returns the human-facing stream as a writer whose failures the UI records.
func (u *UI) Err() io.Writer { return u.err }

// Print writes one line of machine-readable output to the stdout stream.
//
// The write error is dropped here rather than returned: the recorder behind the stream has
// already kept it, and [UI.WriteError] is where a caller collects it.
func (u *UI) Print(s string) { _, _ = fmt.Fprintln(u.out, s) }

// Note writes one line of human-facing text to the stderr stream.
func (u *UI) Note(s string) { _, _ = fmt.Fprintln(u.err, s) }

// Fail writes graft's one-line error report to the stderr stream. A nil error writes
// nothing.
//
// The message is passed through unaltered: every message SPEC.md's failure-mode table names
// already locates its own problem — source "shared": …, graft.toml: …, catalog.yaml: … — so
// a second layer of context would say the same thing twice.
func (u *UI) Fail(err error) {
	if err == nil {
		return
	}
	_, _ = fmt.Fprintln(u.err, "graft:", err)
}

// WriteError returns the first write failure on either stream, or nil.
//
// Write failures are recorded rather than returned so that a report line does not carry
// error handling for a failure that is fatal anyway. The caller checks once, at the end,
// which is what stops graft reporting success while its output went nowhere.
func (u *UI) WriteError() error {
	if u.out.err != nil {
		return u.out.err
	}
	return u.err.err
}

// recorder keeps the first write failure on a stream and otherwise gets out of the way.
type recorder struct {
	w   io.Writer
	err error
}

func (r *recorder) Write(p []byte) (int, error) {
	n, err := r.w.Write(p)
	if err != nil && r.err == nil {
		r.err = err
	}
	return n, err
}
