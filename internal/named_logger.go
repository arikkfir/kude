package internal

import (
	"fmt"
	"log"
)

// NamedLogger creates a logger with a prefix to indent its output.
func NamedLogger(logger *log.Logger, name string) *log.Logger {
	return log.New(logger.Writer(), logger.Prefix()+fmt.Sprintf("[%s]--> ", name), logger.Flags())
}
