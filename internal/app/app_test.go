package app

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		name          string
		args          []string
		wantDevice    string
		wantThreshold int
		wantList      bool
	}{
		{"defaults", nil, "", 30, false},
		{"stable alias", []string{"-k", "usb-receiver-event-kbd", "-t", "25"}, "/dev/input/by-id/usb-receiver-event-kbd", 25, false},
		{"explicit path", []string{"--device", "/dev/input/event7", "--threshold", "0"}, "/dev/input/event7", 0, false},
		{"list", []string{"--list"}, "", 30, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parse(tc.args, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			if got.device != tc.wantDevice || got.threshold != tc.wantThreshold || got.list != tc.wantList {
				t.Fatalf("unexpected options: %+v", got)
			}
		})
	}
}

func TestInvalidArguments(t *testing.T) {
	for _, args := range [][]string{
		{"--threshold", "-1"}, {"--threshold", "1001"}, {"--threshold", "abc"},
		{"--device", "/dev/input/event7", "-k", "receiver"},
		{"-k", "../event7"}, {"-k", "/dev/input/event7"}, {"-k", "."}, {"-k", ".."},
		{"unexpected"}, {"--unknown"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if _, err := parse(args, io.Discard); err == nil {
				t.Fatal("invalid arguments accepted")
			}
		})
	}
}

func TestHelpDoesNotOpenKeyboard(t *testing.T) {
	var output bytes.Buffer
	if err := Run([]string{"--help"}, io.Discard, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "threshold") {
		t.Fatal("help omitted threshold")
	}
}
