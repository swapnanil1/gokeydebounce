//go:build linux && (amd64 || arm64)

package keyboard

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/swapnanil1/gokeydebounce/internal/debounce"
)

// Linux amd64 and arm64 share the little-endian, 64-bit input_event ABI.
const (
	eventSize     = 24
	evSyn         = 0
	evKey         = 1
	evRep         = 0x14
	virtualPrefix = "keydebounce: "
)

type inputEvent struct {
	Sec, Usec  int64
	Type, Code uint16
	Value      int32
}

func (e inputEvent) at() time.Duration {
	return time.Duration(e.Sec)*time.Second + time.Duration(e.Usec)*time.Microsecond
}

func decodeEvent(b []byte) inputEvent {
	return inputEvent{
		Sec:   int64(binary.LittleEndian.Uint64(b)),
		Usec:  int64(binary.LittleEndian.Uint64(b[8:])),
		Type:  binary.LittleEndian.Uint16(b[16:]),
		Code:  binary.LittleEndian.Uint16(b[18:]),
		Value: int32(binary.LittleEndian.Uint32(b[20:])),
	}
}

func writeEvent(dev *os.File, e inputEvent) error {
	var b [eventSize]byte
	// uinput ignores the supplied event timestamp.
	binary.LittleEndian.PutUint16(b[16:], e.Type)
	binary.LittleEndian.PutUint16(b[18:], e.Code)
	binary.LittleEndian.PutUint32(b[20:], uint32(e.Value))
	n, err := dev.Write(b[:])
	if err == nil && n != len(b) {
		return errors.New("short uinput write")
	}
	return err
}

func ioRequest(typ byte, nr int) uintptr { return uintptr(typ)<<8 | uintptr(nr) }
func ior(typ byte, nr, size int) uintptr { return 2<<30 | uintptr(size)<<16 | ioRequest(typ, nr) }
func iow(typ byte, nr, size int) uintptr { return 1<<30 | uintptr(size)<<16 | ioRequest(typ, nr) }
func evBits(typ, size int) uintptr       { return ior('E', 0x20+typ, size) }

func ioctlValue(fd, request, value uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, request, value)
	if errno != 0 {
		return errno
	}
	return nil
}

// Pass pointers directly to Syscall: uintptr must not outlive the call.
func ioctlPointer[T any](fd, request uintptr, value *T) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, request, uintptr(unsafe.Pointer(value)))
	runtime.KeepAlive(value)
	if errno != 0 {
		return errno
	}
	return nil
}

func ioctlBytes(fd, request uintptr, b []byte) error {
	if len(b) == 0 {
		return errors.New("empty ioctl buffer")
	}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, request, uintptr(unsafe.Pointer(&b[0])))
	runtime.KeepAlive(b)
	if errno != 0 {
		return errno
	}
	return nil
}

func inspect(dev *os.File) (string, []byte, error) {
	var name [256]byte
	if err := ioctlBytes(dev.Fd(), ior('E', 0x06, len(name)), name[:]); err != nil {
		return "", nil, err
	}
	end := 0
	for end < len(name) && name[end] != 0 {
		end++
	}
	keys := make([]byte, debounce.KeyCount/8)
	err := ioctlBytes(dev.Fd(), evBits(evKey, len(keys)), keys)
	return string(name[:end]), keys, err
}

func isKeyboard(keys []byte) bool {
	return bit(keys, 30) && bit(keys, 44) && bit(keys, 28) && bit(keys, 57)
}

func createVirtual(source *os.File, name string, keys []byte, repeat bool) (*os.File, error) {
	ui, err := os.OpenFile("/dev/uinput", os.O_WRONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("open /dev/uinput (sudo modprobe uinput; run with sudo): %w", err)
	}
	ok := false
	defer func() {
		if !ok {
			ui.Close()
		}
	}()
	if err := ioctlValue(ui.Fd(), iow('U', 100, 4), evKey); err != nil {
		return nil, err
	}
	for code := 0; code < debounce.KeyCount; code++ {
		if bit(keys, code) {
			if err := ioctlValue(ui.Fd(), iow('U', 101, 4), uintptr(code)); err != nil {
				return nil, err
			}
		}
	}
	if repeat {
		if err := ioctlValue(ui.Fd(), iow('U', 100, 4), evRep); err != nil {
			return nil, err
		}
	}
	var setup [92]byte // input_id (8), name (80), ff_effects_max (4)
	if err := ioctlBytes(source.Fd(), ior('E', 0x02, 8), setup[:8]); err != nil {
		return nil, err
	}
	copy(setup[8:87], virtualPrefix+name) // reserve the final NUL
	if err := ioctlBytes(ui.Fd(), iow('U', 3, len(setup)), setup[:]); err != nil {
		return nil, fmt.Errorf("UI_DEV_SETUP: %w", err)
	}
	if err := ioctlValue(ui.Fd(), ioRequest('U', 1), 0); err != nil {
		return nil, fmt.Errorf("UI_DEV_CREATE: %w", err)
	}
	// Physical repeat events are forwarded. Disable the virtual kernel timer so
	// console clients do not receive both physical and synthesized repeats.
	if repeat {
		for code := uint16(0); code < 2; code++ {
			if err := writeEvent(ui, inputEvent{Type: evRep, Code: code}); err != nil {
				return nil, fmt.Errorf("disable virtual repeat timer: %w", err)
			}
		}
	}
	ok = true
	return ui, nil
}

func waitReadable(fd int) error {
	var read syscall.FdSet
	if fd < 0 || fd >= len(read.Bits)*64 {
		return errors.New("input descriptor exceeds select limit")
	}
	read.Bits[fd/64] |= int64(1) << uint(fd%64)
	timeout := syscall.Timeval{Usec: 100000}
	_, err := syscall.Select(fd+1, &read, nil, nil, &timeout)
	if errors.Is(err, syscall.EINTR) {
		return nil
	}
	return err
}

func bit(bits []byte, code int) bool {
	return code >= 0 && code/8 < len(bits) && bits[code/8]&(1<<uint(code%8)) != 0
}
