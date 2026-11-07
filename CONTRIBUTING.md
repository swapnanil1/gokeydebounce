# Contributing

Use Go 1.22 or newer. No external Go modules or Linux development libraries are
needed. GNU Make is optional; direct Go commands are shown below.

## Layout

- `cmd/keydebounce`: executable entry point and exit handling.
- `internal/app`: argument parsing, validation, and signal handling.
- `internal/debounce`: platform-independent key state machine.
- `internal/keyboard`: Linux device discovery, event encoding, ioctls, and cleanup.
- `packaging/systemd`: optional service template.
- `scripts`: release archive generation.

Keep filtering rules in `internal/debounce` so they can be tested without device
access. Keep raw Linux ABI operations in `internal/keyboard`. Add dependencies
only when the standard library cannot reasonably cover the requirement.

## Development checks

```sh
make check
make fuzz
```

Without Make:

```sh
gofmt -w cmd internal
go vet ./...
go test ./...
CGO_ENABLED=0 go build -trimpath -o bin/keydebounce ./cmd/keydebounce
go test ./internal/debounce -fuzz=FuzzFilterPairing -fuzztime=10s
```

If a C compiler is available, also run `go test -race ./...`. Go uses a C
toolchain for the race detector; normal builds and tests do not require it.

CI tests the declared minimum Go version and the current stable version. It runs
formatting checks, vet, tests, the race detector, Linux amd64/arm64 builds, and a
short fuzz run. CI does not grab input devices or test actual hardware.

For changes to input handling, manually test ordinary typing, intentional double
letters, chords, held-key repetition, Ctrl+C/SIGTERM, and receiver unplug/replug.
Describe which device and compositor were tested when submitting a change.

## Releases

```sh
make dist VERSION=v0.1.0
```

This produces Linux amd64 and arm64 archives and SHA-256 checksums under `dist/`.
Each archive includes the binary, README, MIT license, and service template. It
does not publish anything. Keep generated archives and binaries out of Git.

The module path is `github.com/swapnanil1/gokeydebounce`. The executable is named
`keydebounce`; build it from `./cmd/keydebounce`.

## Contributions and license

Keep changes focused, explain behavior changes, and update relevant tests and
documentation. Contributions are provided under this project's MIT license.
Preserve the existing copyright and permission notices in `LICENSE`.
