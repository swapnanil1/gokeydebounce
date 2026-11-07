package debounce

import (
	"slices"
	"testing"
	"time"
)

type keyEvent struct {
	code  uint16
	value int32
	ms    int
	want  bool
}

func TestFilter(t *testing.T) {
	cases := []struct {
		name      string
		threshold time.Duration
		events    []keyEvent
	}{
		{"chatter and deliberate double tap", 30 * time.Millisecond, []keyEvent{
			{30, 1, 0, true}, {30, 0, 100, true},
			{30, 1, 105, false}, {30, 2, 110, false}, {30, 0, 115, false},
			{30, 1, 131, true}, {30, 2, 170, true}, {30, 0, 180, true},
		}},
		{"keys are independent and chords survive", 30 * time.Millisecond, []keyEvent{
			{42, 1, 0, true}, {30, 1, 1, true}, {30, 0, 2, true},
			{31, 1, 3, true}, {31, 0, 4, true}, {42, 0, 5, true},
		}},
		{"boundary and orphan events", 30 * time.Millisecond, []keyEvent{
			{30, 0, 0, false}, {30, 2, 1, false}, {30, 1, 2, true},
			{30, 1, 3, false}, {30, 0, 4, true},
			{30, 1, 34, false}, {30, 0, 35, false},
			{30, 1, 36, true}, {30, 0, 37, true}, {30, 3, 38, false},
		}},
		{"disabled threshold", 0, []keyEvent{
			{30, 1, 1, true}, {30, 0, 1, true}, {30, 1, 1, true}, {30, 0, 1, true},
		}},
		{"out of range", 30 * time.Millisecond, []keyEvent{
			{KeyCount, 1, 1, false}, {65535, 0, 2, false},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := New(tc.threshold)
			for i, e := range tc.events {
				got := f.Accept(e.code, e.value, time.Duration(e.ms)*time.Millisecond)
				if got != e.want {
					t.Fatalf("event %d: got %v, want %v", i, got, e.want)
				}
			}
		})
	}
}

func TestResetSuppressesHeldKeysUntilRelease(t *testing.T) {
	f := New(30 * time.Millisecond)
	f.Accept(31, 1, 0)
	held := make([]byte, KeyCount/8)
	held[30/8] |= 1 << uint(30%8)
	f.Reset(held)
	if f.keys[31].forwarded {
		t.Fatal("reset retained a forwarded key")
	}
	if f.Accept(30, 2, time.Second) {
		t.Fatal("forwarded repeat of preexisting press")
	}
	if f.Accept(30, 0, time.Second) {
		t.Fatal("forwarded orphan release")
	}
	if !f.Accept(30, 1, time.Second) {
		t.Fatal("new press should be accepted")
	}
}

func TestPressedExcludesSuppressedKeys(t *testing.T) {
	f := New(30 * time.Millisecond)
	f.Accept(31, 1, 0)
	f.Accept(30, 1, 0)
	if got := f.Pressed(); !slices.Equal(got, []uint16{30, 31}) {
		t.Fatalf("pressed keys = %v, want [30 31]", got)
	}
	f.Accept(30, 0, time.Millisecond)
	f.Accept(30, 1, 2*time.Millisecond)
	if got := f.Pressed(); !slices.Equal(got, []uint16{31}) {
		t.Fatalf("pressed keys after chatter = %v, want [31]", got)
	}
	f.Reset(nil)
	if got := f.Pressed(); len(got) != 0 {
		t.Fatalf("pressed keys after reset = %v, want none", got)
	}
}

// For any physical stream, output must remain a sequence of paired presses
// and releases; repeats must only exist inside an accepted press.
func FuzzFilterPairing(f *testing.F) {
	f.Add([]byte{8, 16, 0, 8, 16, 0}) // press, repeat, release, then chatter
	f.Add([]byte{8, 9, 16, 17, 0, 1}) // chord with repeats
	f.Fuzz(func(t *testing.T, stream []byte) {
		filter := New(30 * time.Millisecond)
		var down [8]bool
		for i, b := range stream {
			code, value := uint16(b%8), int32((b/8)%3)
			if !filter.Accept(code, value, time.Duration(i)*time.Millisecond) {
				continue
			}
			switch value {
			case 1:
				if down[code] {
					t.Fatal("duplicate accepted press")
				}
				down[code] = true
			case 0:
				if !down[code] {
					t.Fatal("orphan accepted release")
				}
				down[code] = false
			case 2:
				if !down[code] {
					t.Fatal("orphan accepted repeat")
				}
			}
		}
	})
}
