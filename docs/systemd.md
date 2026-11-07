# Run with systemd

Test the filter manually before enabling automatic startup. The service runs as
root to read/grab the physical keyboard and write to `/dev/uinput`.

Build from source, then install from the repository root:

```sh
CGO_ENABLED=0 go build -trimpath -o bin/keydebounce ./cmd/keydebounce
sudo install -Dm755 bin/keydebounce /usr/local/bin/keydebounce
sudo install -Dm644 packaging/systemd/keydebounce.service /etc/systemd/system/keydebounce.service
sudo systemctl edit --full keydebounce.service
```

For a downloaded release archive, use `keydebounce` instead of `bin/keydebounce`
in the binary installation command; the service template has the same path.

Replace `usb-EXAMPLE_KEYBOARD-event-kbd` in `ExecStart` with the stable filename shown
by `sudo /usr/local/bin/keydebounce --list`. Adjust the threshold if necessary.

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now keydebounce.service
systemctl status keydebounce.service
journalctl -u keydebounce.service
```

The template loads `uinput` before starting. After a failure or receiver
disconnection, it retries every three seconds. A stable `/dev/input/by-id/` path
lets it reopen the same receiver after reconnecting. There is a brief startup
interval before exclusive input filtering resumes.

Stop filtering and disable startup:

```sh
sudo systemctl disable --now keydebounce.service
```

To remove the installed files after disabling the service:

```sh
sudo rm /etc/systemd/system/keydebounce.service
sudo rm /usr/local/bin/keydebounce
sudo systemctl daemon-reload
```
