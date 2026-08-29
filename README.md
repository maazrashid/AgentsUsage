# AgentsUsage

AgentsUsage is a local-first Go service for viewing Claude Code and Codex CLI token usage in one lightweight web dashboard. It reads usage metadata from local JSONL files, calculates API-equivalent cost estimates when a model has known pricing, and serves an embedded dashboard without Node.js, Electron, or a remote backend.

This repository is under active development. The first runnable milestone includes:

- Claude usage parsing from `~/.claude/projects/**/*.jsonl`, including response-snapshot deduplication and cache read/write tokens.
- Codex usage parsing from the allowlisted `~/.codex/sessions/**/*.jsonl` and `~/.codex/archived_sessions/**/*.jsonl` trees, including exact `last_token_usage`, cumulative-counter fallback, and replay suppression.
- Daily, 7-day, all-time, provider, and model aggregates.
- A recursive `fsnotify` watcher with periodic fallback scans.
- `GET /api/status`, `GET /api/stats`, and an embedded responsive dashboard.
- JSON configuration with cross-platform home-directory expansion.
- A native system tray menu with live usage status, dashboard/settings shortcuts, manual refresh, and Start/Stop controls.

Active JSONL files are indexed incrementally: unchanged files are not reopened, appended bytes are parsed from the previous cursor, and incomplete final records are carried into the next refresh. Truncated, replaced, and deleted files rebuild or leave the index cleanly.

## Run locally

Go 1.25 or newer is required.

```bash
go mod download
go run ./cmd/agentsusage
```

The tray icon appears after startup. Left-click it to open the dashboard, or use its menu to start and stop the server, refresh usage, open `config.json`, or quit cleanly. You can also open [http://localhost:8787](http://localhost:8787) directly. By default, the server binds to `0.0.0.0`, so another device on the same trusted network can use `http://<computer-ip>:8787`.

The first run creates a config file in the operating system's user config directory:

- Windows: `%AppData%\agentsusage\config.json`
- macOS: `~/Library/Application Support/agentsusage/config.json`
- Linux: `${XDG_CONFIG_HOME:-~/.config}/agentsusage/config.json`

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8787,
    "autoStart": true
  },
  "paths": {
    "claudeLogs": "~/.claude/projects",
    "codexLogs": "~/.codex"
  },
  "refreshIntervalSeconds": 10
}
```

Use a custom config with:

```bash
go run ./cmd/agentsusage -config /path/to/config.json
```

If `server.autoStart` is false, AgentsUsage opens in the tray with the server stopped. Start it from the tray menu or pass `-start` to override the setting for that launch.

For a headless machine, service manager, or terminal-only session, disable the tray:

```bash
go run ./cmd/agentsusage -no-tray
```

Headless mode requires `server.autoStart` or the `-start` flag because there is no tray menu from which to start the server.

## API

`GET /api/status` reports uptime, last refresh time, and whether a scan warning occurred. `GET /api/stats` returns the aggregate dashboard payload. Raw prompts, responses, session IDs, absolute usage-file paths, and credentials are never included in API responses.

Codex discovery deliberately ignores files outside `sessions` and `archived_sessions`. In particular, AgentsUsage does not read `auth.json`, SQLite databases, browser state, or other Codex configuration.

## Validate and build

```bash
go test ./...
go test -race ./... # where a race-enabled CGO toolchain is available
go build -trimpath -ldflags="-s -w" -o AgentsUsage ./cmd/agentsusage
```

The repository also includes a `Makefile` and a PowerShell release builder:

```bash
make check
make build
```

```powershell
./scripts/build-release.ps1 -Target current
./scripts/build-release.ps1 -Target all
```

The release builder writes stripped Windows amd64, Linux amd64, macOS amd64, and macOS arm64 archives plus `SHA256SUMS` under `dist/`. Pushing a `v*` tag runs the same target matrix in GitHub Actions and publishes the archives to a GitHub release; a manual workflow run builds downloadable artifacts without creating a release.

For a Windows GUI build without a console window:

```powershell
go build -trimpath -ldflags="-s -w -H=windowsgui" -o AgentsUsage.exe ./cmd/agentsusage
```

Cost figures are estimates based on token counts and bundled per-model API prices. They are not subscription charges or invoices. Unknown model names remain unpriced rather than being assigned a guessed rate.

## License

MIT — see [LICENSE](LICENSE).
