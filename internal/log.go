package internal

import "log"

type logWriter struct {
	logger *log.Logger
}

func (l *logWriter) Write(bytes []byte) (n int, err error) {
	l.logger.Print(string(bytes))
	return len(bytes), nil
}
