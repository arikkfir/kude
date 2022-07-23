package internal

import "log"

type LogWriter struct {
	Logger *log.Logger
}

func (l *LogWriter) Write(bytes []byte) (n int, err error) {
	l.Logger.Print(string(bytes))
	return len(bytes), nil
}
