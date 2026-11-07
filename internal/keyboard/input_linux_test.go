//go:build linux && (amd64 || arm64)

package keyboard

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/swapnanil1/gokeydebounce/internal/debounce"
)

func TestEventCodec(t *testing.T) {
	var b [eventSize]byte
	binary.LittleEndian.PutUint64(b[:], 123)
	binary.LittleEndian.PutUint64(b[8:], 456)
	binary.LittleEndian.PutUint16(b[16:], evKey)
	binary.LittleEndian.PutUint16(b[18:], 30)
	binary.LittleEndian.PutUint32(b[20:], 1)
	e := decodeEvent(b[:])
	if e.at() != 123*time.Second+456*time.Microsecond || e.Type != evKey || e.Code != 30 || e.Value != 1 {
		t.Fatalf("bad decode: %+v", e)
	}
	file, err := os.Create(filepath.Join(t.TempDir(), "events"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := writeEvent(file, e); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	got := decodeEvent(b[:])
	if got.Sec != 0 || got.Usec != 0 || got.Type != e.Type || got.Code != e.Code || got.Value != e.Value {
		t.Fatalf("bad encode: %+v", got)
	}
}

func TestLinuxIoctlABI(t *testing.T) {
	for _, tc := range []struct{ got, want uintptr }{
		{ioRequest('U', 1), 0x5501},
		{iow('U', 3, 92), 0x405c5503},
		{iow('E', 0x90, 4), 0x40044590},
		{iow('E', 0xa0, 4), 0x400445a0},
		{evBits(evKey, debounce.KeyCount/8), 0x80604521},
	} {
		if tc.got != tc.want {
			t.Fatalf("ioctl = %#x, want %#x", tc.got, tc.want)
		}
	}
}
