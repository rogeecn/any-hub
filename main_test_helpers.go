package main

import (
	"bytes"
	"testing"
)

// useBufferWriters swaps stdOut/stdErr with in-memory buffers for the duration
// of a test, allowing assertions on CLI output without polluting test logs.
func useBufferWriters(t *testing.T) {
	t.Helper()

	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}

	prevOut := stdOut
	prevErr := stdErr

	stdOut = outBuf
	stdErr = errBuf

	t.Cleanup(func() {
		stdOut = prevOut
		stdErr = prevErr
	})
}

// stdOutBuffer returns the in-use stdout buffer when useBufferWriters is active.
func stdOutBuffer() *bytes.Buffer {
	buf, _ := stdOut.(*bytes.Buffer)
	return buf
}

// stdErrBuffer returns the in-use stderr buffer when useBufferWriters is active.
func stdErrBuffer() *bytes.Buffer {
	buf, _ := stdErr.(*bytes.Buffer)
	return buf
}
