# AgentsUsage

[English](README.md) | [简体中文](https://www.google.com/search?q=README-zh-CN.md)

A ultra-lightweight, cross-platform System Tray background application written in **Go**. **AgentsUsage** parses local telemetry and log files from AI coding assistants—including **Claude CLI** and **Codex CLI**—to monitor token usage, cache performance, and estimated costs in real time.

It runs an embedded HTTP server, allowing you to monitor stats directly from your OS status bar/system tray or access a responsive dashboard from any device (phone, tablet, laptop) on your local network.

---

## Tech Stack & Architecture

* **Core Engine:** Written entirely in pure **Go** for near-instant startup, minimal CPU usage, and tiny memory footprint (~15 MB RAM).
* **System Tray:** Powered by native OS bindings (`fyne.io/systray`) supporting Windows Taskbar, macOS Menu Bar, and Linux AppIndicators.
* **Embedded Web Server:** High-performance REST API and static asset server built using Go's `net/http` standard library.
* **File System Monitoring:** Efficient real-time log tracking via Go's native OS file notification watchers (`fsnotify`).

---

## Key Features

* **Multi-Agent Tracking**: Automatically detects and parses local logs from both **Claude CLI** and **Codex CLI**.
* **System Tray Control**:
* Displays quick token usage and daily cost summaries in the menu preview.
* Start, stop, or restart the embedded HTTP server with one click.
* Quick action to open the dashboard in your default browser.


* **Local Network Web Dashboard**:
* Serves a mobile-friendly web UI bound to local IP (`0.0.0.0`).
* Access stats from your smartphone or tablet over local Wi-Fi.


* **Detailed Analytics**:
* Daily, weekly, and total token breakdowns.
* Model-specific cost estimations and token allocation.
* Cache hit rate performance.


* **Single Portable Binary**: Zero external runtime dependencies (no Node.js, Electron, or Python required).

---

## Preview

```text
┌─────────────────────────────────────────┐
│ AgentsUsage (System Tray / Menu Bar)    │
├─────────────────────────────────────────┤
│ Status: Server Running (Port 8787)      │
│ Today: 142.5k tokens ($0.42)            │
├─────────────────────────────────────────┤
│ 🌐 Open Dashboard                       │
│ ▶️  Start Server                        │
│ ⏹️  Stop Server                         │
│ ⚙️  Settings...                         │
├─────────────────────────────────────────┤
│ ❌ Quit                                 │
└─────────────────────────────────────────┘

```

---

## Installation & Setup

### Download Pre-built Binaries

Go to the [Releases](https://www.google.com/search?q=https://github.com/maazrashid/AgentsUsage/releases) page and download the single standalone binary for your platform:

* **Windows:** `AgentsUsage-windows-amd64.exe` (or ZIP)
* **macOS:** `AgentsUsage-darwin-universal.dmg` (or binary)
* **Linux:** `AgentsUsage-linux-amd64.tar.gz`

### Quick Start

1. **Run the Application**: Double-click the executable file. The tray icon will appear:
* **Windows**: Bottom-right notification area.
* **macOS**: Top-right menu bar.
* **Linux**: Top or bottom panel (AppIndicator area).


2. **Access the Dashboard**:
* **Local PC**: Click the tray icon and select **Open Dashboard**, or visit `http://localhost:8787`.
* **Mobile Device**: Ensure your phone is connected to the same Wi-Fi network and navigate to `http://<YOUR-PC-IP>:8787`.



---

## Configuration

The application creates a default `config.json` next to the executable (or under `~/.config/agentsusage/`) on first run:

```json
{
  "server": {
    "port": 8787,
    "host": "0.0.0.0",
    "autoStart": true
  },
  "paths": {
    "claudeLogs": "~/.claude/projects",
    "codexLogs": "~/.codex/logs"
  },
  "refreshIntervalSeconds": 10
}

```

---

## Building from Source

### Prerequisites

* **Go** 1.21 or higher installed on your system.

### Build Commands

```bash
# Clone repository
git clone https://github.com/maazrashid/AgentsUsage.git
cd AgentsUsage

# Install Go module dependencies
go mod download

# Run directly
go run main.go

# Build production binary for current OS
go build -ldflags="-s -w" -o AgentsUsage

```

### Cross-Compilation

Go makes targeting other operating systems seamless from a single machine:

```bash
# Build for Windows (from Linux/macOS)
GOOS=windows GOARCH=amd64 go build -o AgentsUsage.exe

# Build for macOS (Universal Binary)
GOOS=darwin GOARCH=arm64 go build -o AgentsUsage-arm64
GOOS=darwin GOARCH=amd64 go build -o AgentsUsage-amd64
lipo -create -output AgentsUsage-mac AgentsUsage-arm64 AgentsUsage-amd64

# Build for Linux
GOOS=linux GOARCH=amd64 go build -o AgentsUsage-linux

```

---

## Contributing

Contributions, bug reports, and PRs are standard practice! Feel free to check the [issues page](https://www.google.com/search?q=https://github.com/maazrashid/AgentsUsage/issues).

1. Fork the Repository
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

---

## License

Distributed under the MIT License. See `LICENSE` for details.