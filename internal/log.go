package internal

import "log"

type LogWriter struct {
	Logger *log.Logger
}

func (l *LogWriter) Write(bytes []byte) (n int, err error) {
	l.Logger.Print(string(bytes))
	return len(bytes), nil
}

// TODO: support named prefix, e.g. for function names

// ChildLogger creates a logger with a prefix to indent its output.
func ChildLogger(logger *log.Logger) *log.Logger {
	var prefix string
	if logger.Prefix() == "" {
		prefix = "-> "
	} else {
		prefix = "---" + logger.Prefix()
	}
	return log.New(logger.Writer(), prefix, logger.Flags())
}
