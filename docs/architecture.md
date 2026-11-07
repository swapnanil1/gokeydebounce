# Architecture

`cmd/keydebounce` invokes `internal/app`, which parses and validates options and
arranges signal cancellation. `internal/keyboard` discovers the physical event
device, configures a virtual keyboard, and runs the Linux event loop. It delegates
key decisions to the platform-independent `internal/debounce` state machine.

The project uses only the Go standard library. Linux-specific files use build
constraints for amd64 and arm64. Other platforms can display CLI help, but input
operations return an unsupported-platform error.

## State machine

Each key tracks whether it is physically down, whether that press was forwarded,
and the monotonic timestamp of its last accepted release.

1. A first press is forwarded immediately.
2. An accepted release records the debounce window's starting time.
3. Another press at or inside the window is suppressed.
4. Its release and repeats are also suppressed, preserving output pairing.
5. Suppressed presses do not extend the debounce window.

Accepted held-key repeats are forwarded. The virtual device's kernel repeat timer
is disabled to avoid adding another repeat source. Desktops and applications
may also implement their own repeat timing.

## Device lifecycle

The virtual keyboard is created before taking exclusive physical-device access.
A one-second startup interval allows compositor discovery and release of the key
that launched the command. Pre-grab events are drained, and currently held keys
are suppressed until their release.

Input events use the Linux 64-bit little-endian ABI. The physical event clock is
set to `CLOCK_MONOTONIC`. Accepted key events retain the original report boundaries;
scan-code metadata is discarded.

On `SYN_DROPPED`, events are ignored through the next synchronization report.
Accepted virtual keys are released, the physical held-key bitmap is queried,
and the filter resets. Keys already held remain suppressed until released.

On normal termination, errors, or signal cancellation, cleanup releases virtual
held keys, ungrabs the physical device, and destroys the virtual device. SIGKILL
cannot run Go cleanup, but closing process descriptors releases the kernel grab
and virtual device.

Receiver removal returns an error to the caller. The CLI exits, while the optional
systemd unit can retry and reopen a stable device alias. No background retry loop
is hidden inside the filtering package.
