// Package app implements the keydebounce command-line interface.
package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/swapnanil1/gokeydebounce/internal/keyboard"
)

type options struct {
	device    string
	keyboard  string
	threshold int
	list      bool
}

func parse(args []string, stderr io.Writer) (options, error) {
	var o options
	flags := flag.NewFlagSet("keydebounce", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&o.device, "device", "", "keyboard event path (prefer /dev/input/by-id/...-event-kbd)")
	flags.StringVar(&o.keyboard, "keyboard", "", "keyboard filename under /dev/input/by-id")
	flags.StringVar(&o.keyboard, "k", "", "alias for --keyboard")
	flags.IntVar(&o.threshold, "threshold", 30, "release-to-press debounce window in milliseconds (0 disables)")
	flags.IntVar(&o.threshold, "t", 30, "alias for --threshold")
	flags.BoolVar(&o.list, "list", false, "list keyboard event paths and stable aliases")
	if err := flags.Parse(args); err != nil {
		return o, err
	}
	if flags.NArg() != 0 {
		return o, errors.New("unexpected arguments; use --help")
	}
	if o.threshold < 0 || o.threshold > 1000 {
		return o, errors.New("threshold must be between 0 and 1000 ms")
	}
	if o.device != "" && o.keyboard != "" {
		return o, errors.New("choose --device or --keyboard, not both")
	}
	if o.keyboard != "" {
		if filepath.Base(o.keyboard) != o.keyboard || o.keyboard == "." || o.keyboard == ".." {
			return o, errors.New("--keyboard expects a filename; use --device for a full path")
		}
		o.device = filepath.Join("/dev/input/by-id", o.keyboard)
	}
	return o, nil
}

// Run parses arguments, lists devices or runs the filter, and handles shutdown.
// Help exits successfully without opening an input device.
func Run(args []string, stdout, stderr io.Writer) error {
	o, err := parse(args, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	if o.list {
		return keyboard.List(stdout)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := keyboard.Run(ctx, o.device, time.Duration(o.threshold)*time.Millisecond, stderr); err != nil {
		return fmt.Errorf("filter: %w", err)
	}
	return nil
}
