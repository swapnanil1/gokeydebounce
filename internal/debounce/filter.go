package debounce

import "time"

// KeyCount is the number of key codes in the Linux keyboard ABI.
const KeyCount = 0x300

type keyState struct {
	down      bool
	forwarded bool
	lastUp    time.Duration
	hasUp     bool
}

// Filter tracks accepted and suppressed key presses. A rejected press stays
// suppressed until its release, including repeats. Only an accepted release
// starts the debounce window.
type Filter struct {
	threshold time.Duration
	keys      [KeyCount]keyState
}

// Accept reports whether a key event should reach the virtual keyboard.
// Values are 0 (release), 1 (press), and 2 (repeat); at must be monotonic.
func (f *Filter) Accept(code uint16, value int32, at time.Duration) bool {
	if int(code) >= len(f.keys) {
		return false
	}
	k := &f.keys[code]
	switch value {
	case 1:
		if k.down {
			return false
		}
		k.down = true
		k.forwarded = !k.hasUp || f.threshold == 0 || at-k.lastUp > f.threshold
		return k.forwarded
	case 0:
		if !k.down {
			return false
		}
		k.down = false
		if !k.forwarded {
			return false
		}
		k.forwarded = false
		k.lastUp, k.hasUp = at, true
		return true
	case 2:
		return k.down && k.forwarded
	}
	return false
}

// Reset clears the filter and suppresses keys already held in the bitmap.
func (f *Filter) Reset(held []byte) {
	f.keys = [KeyCount]keyState{}
	for code := range f.keys {
		if bit(held, code) {
			f.keys[code].down = true
		}
	}
}

func bit(bits []byte, code int) bool {
	return code >= 0 && code/8 < len(bits) && bits[code/8]&(1<<uint(code%8)) != 0
}

// New constructs a filter with a release-to-press debounce window.
func New(threshold time.Duration) *Filter { return &Filter{threshold: threshold} }

// Pressed returns the codes currently held down on the virtual keyboard.
func (f *Filter) Pressed() []uint16 {
	var codes []uint16
	for code, state := range f.keys {
		if state.forwarded {
			codes = append(codes, uint16(code))
		}
	}
	return codes
}
