# keydebounce

A standalone keyboard chatter filter for Linux. Supports wired keyboards and
wireless receivers exposed as Linux keyboard devices.

Supports Linux amd64 and arm64. Requires Go 1.22+ to build, with no third-party
Go modules or shared-library dependencies.

## Build

Install Go 1.22 or newer using the package manager for the target distribution.

Clone the source (requires Git):

```sh
git clone https://github.com/swapnanil1/gokeydebounce.git
cd gokeydebounce
```

Build from the repository root:

```sh
go test ./...
CGO_ENABLED=0 go build -trimpath -o bin/keydebounce ./cmd/keydebounce
```

If GNU Make is available, `make check` runs formatting checks, vet, tests, and the
build. The executable is written to `bin/keydebounce`.

## Identify your keyboard

```sh
sudo modprobe uinput
sudo ./bin/keydebounce --list
```

Example output (your device name will differ):

```text
/dev/input/event7  Example Keyboard
  /dev/input/by-id/usb-EXAMPLE_KEYBOARD-event-kbd
```

Use the full `/dev/input/by-id/...-event-kbd` path. Unlike `event7`, the stable alias
normally survives reconnecting the same receiver. It can differ for another
receiver or connection mode. Choose the main keyboard node, rather than a
separate media-control node. Combined keyboard/pointer nodes are rejected.

## Run

```sh
sudo ./bin/keydebounce --device /dev/input/by-id/usb-EXAMPLE_KEYBOARD-event-kbd --threshold 30
```

The filename-only shorthand is also supported:

```sh
sudo ./bin/keydebounce -k usb-EXAMPLE_KEYBOARD-event-kbd -t 30
```

Leave the terminal open and release all keys during startup. Press Ctrl+C in that
terminal to stop. Desktop key-repeat settings are independent of the debounce
threshold.

| Option | Purpose |
| --- | --- |
| `--list` | List physical keyboards and stable aliases |
| `--device PATH` | Select an event device by its full path |
| `--keyboard NAME`, `-k NAME` | Select a filename under `/dev/input/by-id/` |
| `--threshold MS`, `-t MS` | Release-to-press interval in milliseconds; default 30 |
| `--help` | Show command usage |

Without an explicit device, automatic selection requires exactly one detected
keyboard. Run only one input filter against a physical keyboard at a time.

## How filtering works

```text
Keyboard / wireless receiver → evdev → keydebounce → uinput → desktop / applications
```

The program takes exclusive access to one keyboard and creates a virtual one.
A press of the same key at or within 30 ms of its last accepted release is
discarded, together with its matching release and held-key repeats. Other keys
are independent. Accepted presses are forwarded immediately.

Lower the threshold if intentional double letters disappear; raise it gradually
if chatter remains. `--threshold 0` disables time filtering. This is a
release-to-press interval, not the interval between two presses.

Turning the keyboard off/on while its receiver remains connected usually leaves
the device available. Unplugging the receiver ends a manually started filter;
restart it after reconnecting. The optional systemd service retries automatically.

## Service and development

- [Systemd installation and removal](docs/systemd.md)
- [Architecture and event handling](docs/architecture.md)
- [Development checks and release packaging](CONTRIBUTING.md)

## Troubleshooting

`permission denied`: run the command with `sudo`. The filter needs access to the
physical input device and `/dev/uinput`.

`device has non-keyboard capabilities`: try the receiver's other keyboard entry
from `--list`. Some receivers expose a main keyboard interface and another with
extra input axes. Select an entry that exposes only keyboard events. The filter
requires a keyboard-only event node.

`unexpected arguments`: use either `--device /dev/input/by-id/NAME` or `-k NAME`.
The `-k` option takes a filename; it is not a switch to combine with `--device`.

`Filtering ... at 30ms` means the keyboard was grabbed and the event loop is
running. The program logs startup and errors, not individual keystrokes or counts
of discarded presses. To check filtering, temporarily use `--threshold 300` and
quickly tap the same letter twice in another window. Only one should appear.
Stop the filter to compare, then return to a suitable threshold such as 30 ms.

## Limitations

The filter forwards keyboard events, not a complete clone of every input
capability. Hardware LED feedback, scan-code metadata, pointer axes, and force
feedback are not proxied. Caps/Num Lock keys still work in applications, but
their physical LED updates are not relayed.

The virtual keyboard name starts with `keydebounce: `. Per-device desktop
configuration targeting the physical keyboard may need to target the virtual
keyboard instead.

Automated tests do not replace testing with an actual receiver and compositor.

## License

MIT. See [LICENSE](LICENSE) for the copyright and permission notices.
