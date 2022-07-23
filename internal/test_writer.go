package internal

import "testing"

type TestWriter struct {
	T *testing.T
}

func (tw *TestWriter) Write(p []byte) (n int, err error) {
	tw.T.Helper()
	tw.T.Logf("%s", p)
	return len(p), nil
}
