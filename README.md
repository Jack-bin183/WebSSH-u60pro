# WebSSH-u60pro

A web-based management tool built specifically for the **U60Pro**, providing an in-browser SSH terminal, SFTP file manager, transparent proxy control, and direct access to router-native interfaces. Deploys as a single binary with no external dependencies.

---

## Features

### Web SSH Terminal
- Full-featured in-browser terminal (xterm.js) over WebSocket-proxied SSH sessions
- Manage multiple saved connection profiles; credentials stored AES-encrypted
- Automatic terminal resize synchronisation
- Execute commands across sessions; disconnect individual sessions on demand

### SFTP File Manager
- Browse directories, upload and download files
- Create directories and files, rename, delete, change permissions (chmod)
- Edit text files directly in the browser
- Compress directories and extract archives

### Built-in SSHD Server
- Manage local SSHD user accounts (CRUD)
- Manage SSH certificates (authorized_keys) with plain-text export
- Start/stop service control

### Mihomo Transparent Proxy
- **One-click install / uninstall**: downloads the Mihomo ARM64 binary and control script `mm.sh` from a GitHub Release, with automatic proxy-URL fallback for faster downloads
- **Start / stop / restart / reload ipset**: controls the transparent proxy via `mm.sh` with live output streaming
- **Data file update**: one-click update of `chnroute.txt`, `chnroute6.txt`, `GeoSite.dat`, and `geoip.metadb`, with progress bar and cancellation support
- **Boot autostart**: on webssh startup, detects a marker file and runs `mm.sh start` in the background if autostart is enabled
- **Inline config editor**: edit `config.yaml` directly in the browser; automatic backup before saving
- **Status overview**: running state, PID, uptime, API reachability, version info, and data-file inventory

### ZTE Router-Specific Interfaces
- **UBUS JSON-RPC proxy**: batch-call router low-level UBUS interfaces
- **WiFi power-saving mode (PSM)**: read and toggle WiFi power-save / high-performance mode
- **WiFi radio switch**: independently toggle 2.4 GHz / 5 GHz radios
- **Network AMBR**: read current AMBR rate-limit configuration

### Auto-Update
- Checks the latest version from GitHub Releases with multiple proxy-URL fallbacks
- Downloads the new binary in the background, then replaces and restarts; full progress display

### Access Control (NetFilter)
- IP allowlist / blocklist rule management enforced at the middleware layer

### Miscellaneous
- **Command notes**: save frequently used command snippets and paste them into the SSH terminal in one click
- **Policy configuration**: CRUD management for custom policy rules
- **Login audit**: records login events with searchable history
- **Open ADB**: triggers the ADB debug interface via `usb_switch`
- **TLS support**: place `cert.pem` + `key.key` in the working directory to enable HTTPS automatically

---

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.21+, vendored gin / GORM / SQLite / WebSocket / crypto-ssh / sftp |
| Frontend | Vue 3 + TypeScript + Vite + Element Plus |
| Database | SQLite (default `gowebssh.db`, stored in the working directory) |
| Config file | TOML at `~/.GoWebSSH/GoWebSSH.toml` (or override with `-ConfigDir`) |
| Deployment target | Linux ARM64 (ZTE U60Pro, OpenWrt kernel) |

---

## Build & Deploy

### Requirements
- Go 1.21+
- Node.js 18+ (frontend build)
- `upx` (optional, for binary compression)

### One-liner (set VERSION as needed)

```bash
cd webssh && npm install && npm run build \
  && rsync -a --delete dist/ ../gossh/webroot/ \
  && cd ../gossh \
  && VERSION=20260514_1414 \
  && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
     go build -ldflags="-s -w -X main.version=${VERSION}" -o webssh \
  && upx --best --lzma webssh
```

### Step by step

```bash
# 1. Build the frontend
cd webssh
npm install
npm run build

# 2. Sync static assets into the backend
rsync -a --delete dist/ ../gossh/webroot/

# 3. Cross-compile the backend (target: ARM64 Linux)
cd ../gossh
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
  go build -ldflags="-s -w -X main.version=dev" -o webssh

# 4. (Optional) Compress with UPX
upx --best --lzma webssh
```

### Upload and run

```bash
# Copy the binary to the router
scp webssh root@<router-ip>:/data/plugins/WebSSH/

# Run on the router (SQLite database is stored in the working directory)
cd /data/plugins/WebSSH && ./webssh
```

Listens on `:8899` by default. A setup wizard runs on the first visit.

---

## Mihomo Transparent Proxy

All Mihomo-related files live under `/data/plugins/mihomo/` on the router.

### First-time setup

1. In the WebSSH UI → **Mihomo** → **Install** tab, click "Install Mihomo"
   - Automatically downloads `mihomo-linux-arm64` and `mm.sh` from the `latest-data` GitHub Release
2. Switch to the **Config** tab and paste your `config.yaml` (Clash Meta format)
3. Return to the **Overview** tab and click "Start"

### Keeping data files current

- **Automatic**: a GitHub Actions workflow rebuilds the Release every day at UTC 02:00 with the latest chnroute / GeoSite / GeoIP data and Mihomo binary
- **Manual**: UI → **Data Update** tab → "Update Now"

### Boot autostart

Enable the "Boot autostart" toggle at the bottom of the Overview tab. After a router reboot, webssh will automatically call `mm.sh start` in the background to bring Mihomo up.

---

## Frontend Development

```bash
cd webssh
npm install
npm run dev
# Open http://127.0.0.1:3000/ — hot reload enabled
```

---

## Repository Layout

```
gossh/           Go backend (all dependencies vendored locally)
  main.go        Entry point, route registration, auto-update logic
  app/config/    Configuration loading
  app/model/     Database models
  app/service/   Business logic (SSH, SFTP, SSHD, Mihomo, ZTE UBUS, …)
  webroot/       Compiled frontend static assets
webssh/          Vue 3 frontend source
mihomo/          mm.sh proxy control script
.github/
  workflows/
    update-mihomo-data.yml   Daily automated Mihomo data release workflow
```

---

## License

[MIT](LICENSE)
