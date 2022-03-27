package test

import (
	"bytes"
	"io"
	"os"
)

func Capture(captureStdout, captureStderr bool, f func()) (stdout, stderr string) {
	var stdoutReader, stderrReader io.Reader
	var oldStdout, oldStderr *os.File
	var stdoutWriter, stderrWriter *os.File
	var err error

	if captureStdout {
		stdoutReader, stdoutWriter, err = os.Pipe()
		if err != nil {
			panic(err)
		}
		oldStdout = os.Stdout
		os.Stdout = stdoutWriter
		defer func() { os.Stdout = oldStdout }()
	}

	if captureStderr {
		stderrReader, stderrWriter, err = os.Pipe()
		if err != nil {
			panic(err)
		}
		oldStderr = os.Stderr
		os.Stderr = stderrWriter
		defer func() { os.Stderr = oldStderr }()
	}

	f()

	if stdoutWriter != nil {
		stdoutWriter.Close()
		var stdoutBuffer bytes.Buffer
		if _, err := io.Copy(&stdoutBuffer, stdoutReader); err != nil {
			panic(err)
		}
		stdout = stdoutBuffer.String()
	}

	if stderrWriter != nil {
		stderrWriter.Close()
		var stderrBuffer bytes.Buffer
		if _, err := io.Copy(&stderrBuffer, stderrReader); err != nil {
			panic(err)
		}
		stderr = stderrBuffer.String()
	}

	return stdout, stderr
}
