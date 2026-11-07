//go:build !linux || (!amd64 && !arm64)

package keyboard

import (
	"context"
	"errors"
	"io"
	"time"
)

var errUnsupported = errors.New("keydebounce supports Linux amd64 and arm64 only")

func List(io.Writer) error                                        { return errUnsupported }
func Run(context.Context, string, time.Duration, io.Writer) error { return errUnsupported }
