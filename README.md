# AgentsUsage

AgentsUsage is a local-first Go service for viewing Claude Code and Codex CLI token usage in one lightweight web dashboard. It reads usage metadata from local JSONL files, calculates API-equivalent cost estimates when a model has known pricing, and serves an embedded dashboard without Node.js, Electron, or a remote backend.

This repository is under active development. The first runnable milestone includes:

- Claude usage parsing from `~/.claude/projects/**/*.jsonl`, including response-snapshot deduplication and cache read/write tokens.
- Codex usage parsing from the allowlisted `~/.codex/sessions/**/*.jsonl` and `~/.codex/archived_sessions/**/*.jsonl` trees, including exact `last_token_usage`, cumulative-counter fallback, and replay suppression.
- Daily, 7-day, all-time, provider, and model aggregates.
- A recursive `fsnotify` watcher with periodic fallback scans.
- `GET /api/status`, `GET /api/stats`, and an embedded responsive dashboard.
- JSON configuration with cross-platform home-directory expansion.

System tray controls and release packaging are the next implementation phase.

## Run locally

Go 1.22 or newer is required.

```bash
go mod download
go run ./cmd/agentsusage
```

Open [http://localhost:8787](http://localhost:8787). By default, the server binds to `0.0.0.0`, so another device on the same trusted network can use `http://<computer-ip>:8787`.

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

If `server.autoStart` is false, pass `-start` to explicitly launch the standalone service. That setting will be controlled by the tray menu once tray integration lands.

## API

`GET /api/status` reports uptime, last refresh time, and whether a scan warning occurred. `GET /api/stats` returns the aggregate dashboard payload. Raw prompts, responses, session IDs, absolute usage-file paths, and credentials are never included in API responses.

Codex discovery deliberately ignores files outside `sessions` and `archived_sessions`. In particular, AgentsUsage does not read `auth.json`, SQLite databases, browser state, or other Codex configuration.

## Validate and build

```bash
go test ./...
go test -race ./... # where a race-enabled CGO toolchain is available
go build -trimpath -ldflags="-s -w" -o AgentsUsage ./cmd/agentsusage
```

Cost figures are estimates based on token counts and bundled per-model API prices. They are not subscription charges or invoices. Unknown model names remain unpriced rather than being assigned a guessed rate.

## License

MIT — see [LICENSE](LICENSE).
