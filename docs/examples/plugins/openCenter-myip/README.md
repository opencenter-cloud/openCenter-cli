# openCenter-myip (Go example plugin)

A minimal Go plugin for `openCenter` that fetches your public IP from `https://icanhazip.com/` and prints it.

## Build

```
cd docs/examples/plugins/openCenter-myip
go build -o openCenter-myip .
```

## Install

Place the compiled binary where `openCenter` discovers plugins (pick one):

- `~/.config/openCenter/plugins` (default config-dir on macOS/Linux)
- Directory pointed to `OPENCENTER_PLUGINS_DIR`
- Any directory on your `PATH`

Example:

```
mkdir -p ~/.config/openCenter/plugins
mv ./openCenter-myip ~/.config/openCenter/plugins/
chmod +x ~/.config/openCenter/plugins/openCenter-myip
```

## Run

```
openCenter myip
```

The command performs an HTTPS GET to `https://icanhazip.com/` with a short timeout and prints the resulting IP address.

## Notes

- Uses only the Go standard library (`net/http`).
- Sets a 5s timeout and a custom `User-Agent`.
- Prints a single trimmed line; errors go to stderr with a non-zero exit code.

