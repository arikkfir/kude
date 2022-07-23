package kude

import (
	"log"
)

func NewExecution(p Pipeline, logger *log.Logger) (Execution, error) {
	return &executionImpl{
		pipeline: p,
		logger:   logger,
	}, nil
}
