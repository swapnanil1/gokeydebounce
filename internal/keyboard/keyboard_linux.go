//go:build linux && (amd64 || arm64)

// Package keyboard manages Linux keyboard discovery and evdev/uinput filtering.
package keyboard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/swapnanil1/gokeydebounce/internal/debounce"
)

type keyboardInfo struct{ path, name string }

func keyboards() ([]keyboardInfo, error) {
	paths, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return nil, err
	}
	var devices []keyboardInfo
	var denied bool
	for _, path := range paths {
		dev, err := os.Open(path)
		if err != nil {
			denied = denied || errors.Is(err, os.ErrPermission)
			continue
		}
		name, keys, err := inspect(dev)
		dev.Close()
		if err == nil && isKeyboard(keys) && !strings.HasPrefix(name, virtualPrefix) {
			devices = append(devices, keyboardInfo{path, name})
		}
	}
	if len(devices) == 0 && denied {
		return nil, errors.New("cannot read input devices; run with sudo")
	}
	return devices, nil
}

// List prints physical keyboards and their stable device aliases.
func List(out io.Writer) error {
	devices, err := keyboards()
	if err != nil {
		return err
	}
	aliases, _ := filepath.Glob("/dev/input/by-id/*")
	for _, dev := range devices {
		fmt.Fprintf(out, "%s  %s\n", dev.path, dev.name)
		for _, alias := range aliases {
			target, err := filepath.EvalSymlinks(alias)
			if err == nil && target == dev.path {
				fmt.Fprintf(out, "  %s\n", alias)
			}
		}
	}
	if len(devices) == 0 {
		fmt.Fprintln(out, "No keyboard event devices found.")
	}
	return nil
}

// Run filters a keyboard until cancellation, disconnection, or an I/O error.
// If path is empty, exactly one detected keyboard is required.
func Run(ctx context.Context, path string, threshold time.Duration, log io.Writer) error {
	if path == "" {
		devices, err := keyboards()
		if err != nil {
			return err
		}
		if len(devices) != 1 {
			return fmt.Errorf("found %d keyboards; use sudo keydebounce --list, then --device PATH", len(devices))
		}
		path = devices[0].path
	}
	dev, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return fmt.Errorf("open keyboard (try sudo): %w", err)
	}
	defer dev.Close()
	fd := dev.Fd()
	name, keys, err := inspect(dev)
	if err != nil {
		return err
	}
	if strings.HasPrefix(name, virtualPrefix) {
		return errors.New("refusing to filter a keydebounce virtual keyboard")
	}
	if !isKeyboard(keys) {
		return errors.New("selected device is not a keyboard")
	}
	// Do not silently swallow a mouse/touchpad bundled into the same event node.
	types := make([]byte, 4)
	if err := ioctlBytes(fd, evBits(0, len(types)), types); err != nil {
		return err
	}
	for _, typ := range []int{2, 3, 5, 0x12, 0x15} {
		if bit(types, typ) {
			return errors.New("device has non-keyboard capabilities; select its keyboard-only event node")
		}
	}
	clock := int32(1) // CLOCK_MONOTONIC, shared by every timestamp in this stream.
	if err := ioctlPointer(fd, iow('E', 0xa0, 4), &clock); err != nil {
		return fmt.Errorf("set monotonic event clock: %w", err)
	}
	ui, err := createVirtual(dev, name, keys, bit(types, evRep))
	if err != nil {
		return err
	}
	defer ui.Close()
	// File.Fd may restore blocking mode. Set this after the last source Fd call.
	if err := syscall.SetNonblock(int(fd), true); err != nil {
		return err
	}
	f := debounce.New(threshold)
	grabbed := false
	defer func() {
		// Releasing the physical device happens even when cleanup writes fail.
		_ = releaseKeys(ui, f)
		if grabbed {
			_ = ioctlValue(fd, iow('E', 0x90, 4), 0)
		}
		_ = ioctlValue(ui.Fd(), ioRequest('U', 2), 0)
	}()
	fmt.Fprintf(log, "Preparing %s; release all keys. Ctrl+C stops.\n", name)
	// Allow the compositor to discover the virtual keyboard before taking input.
	select {
	case <-ctx.Done():
		return nil
	case <-time.After(time.Second):
	}
	if err := ioctlValue(fd, iow('E', 0x90, 4), 1); err != nil {
		return fmt.Errorf("grab keyboard: %w", err)
	}
	grabbed = true
	// Discard pre-grab events, then suppress keys already held at startup.
	buf := make([]byte, eventSize*64)
	for {
		if ctx.Err() != nil {
			return nil
		}
		n, err := syscall.Read(int(fd), buf)
		if errors.Is(err, syscall.EAGAIN) {
			break
		}
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.EOF
		}
	}
	if err := resync(fd, ui, f); err != nil {
		return err
	}
	fmt.Fprintf(log, "Filtering %s at %s.\n", path, threshold)
	dropped := false
	for ctx.Err() == nil {
		if err := waitReadable(int(fd)); err != nil {
			return err
		}
		n, err := syscall.Read(int(fd), buf)
		if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read keyboard (receiver disconnected?): %w", err)
		}
		if n == 0 {
			return io.EOF
		}
		if n%eventSize != 0 {
			return errors.New("incomplete Linux input event")
		}
		for offset := 0; offset < n; offset += eventSize {
			e := decodeEvent(buf[offset : offset+eventSize])
			if e.Type == evSyn && e.Code == 3 {
				dropped = true
				continue
			}
			if dropped {
				if e.Type == evSyn && e.Code == 0 {
					if err := resync(fd, ui, f); err != nil {
						return err
					}
					dropped = false
				}
				continue
			}
			if e.Type == evSyn && e.Code == 0 {
				if err := writeEvent(ui, inputEvent{}); err != nil {
					return err
				}
			} else if e.Type == evKey && f.Accept(e.Code, e.Value, e.at()) {
				if err := writeEvent(ui, e); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func resync(fd uintptr, ui *os.File, f *debounce.Filter) error {
	if err := releaseKeys(ui, f); err != nil {
		return err
	}
	held := make([]byte, debounce.KeyCount/8)
	if err := ioctlBytes(fd, ior('E', 0x18, len(held)), held); err != nil {
		return fmt.Errorf("read held keys: %w", err)
	}
	f.Reset(held)
	return nil
}

func releaseKeys(ui *os.File, f *debounce.Filter) error {
	for _, code := range f.Pressed() {
		if err := writeEvent(ui, inputEvent{Type: evKey, Code: code}); err != nil {
			return err
		}
	}
	return writeEvent(ui, inputEvent{})
}
