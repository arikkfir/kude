package internal

import (
	"context"
	"io"
)

type Function interface {
	GetName() string
	Invoke(ctx context.Context, r io.Reader, w io.Writer) error
}
