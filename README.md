# WebSSH-u60pro

WebSSH-u60pro is a lightweight web management tool for the ZTE U60Pro. It provides browser-based SSH access, SFTP file management, device status panels, router-specific network controls, and common maintenance tools in a single deployable binary.

The project is designed for ARM64 Linux devices and stores its local data in SQLite, so it can run directly on the router without an external database.

## Features

### Web SSH Terminal

- Browser terminal powered by xterm.js and WebSocket SSH sessions
- Multiple saved connection profiles
- AES-encrypted credential storage
- Automatic terminal resize synchronization
- Multi-session command execution
- Per-session reconnect, disconnect, and terminal clear controls
- Command notes for saving frequently used snippets

### SFTP File Manager

- Browse remote directories
- Upload and download files
- Create files and directories
- Rename, delete, and change permissions
- Edit small text files directly in the browser
- Compress directories and extract supported archives

### Built-in SSHD Management

- Manage local SSHD users
- Manage authorized SSH certificates
- Enable, disable, edit, and delete SSHD records from the web UI

### U60Pro Device Panels

- Device uptime, connection state, battery, and network status
- 4G / 5G signal details and carrier information
- UBUS JSON-RPC access for router-native data
- WiFi power-saving / high-performance mode control
- 2.4 GHz and 5 GHz radio controls
- Network AMBR status

### System Tools

- Open ADB debug interface
- Local speed test
- SMS forwarding controls
- Device native UI service controls
- File permission helper
- Login audit history
- Access control rules

### Updates And Security

- Web-based update check and install flow
- Download progress and cancellation support
- IP allowlist / blocklist middleware
- First-run initialization wizard
- Optional HTTPS when `cert.pem` and `key.key` are placed in the working directory

## Tech Stack

| Layer | Technology |
| --- | --- |
| Backend | Go 1.21+, vendored Gin / GORM / SQLite / WebSocket / crypto-ssh / SFTP |
| Frontend | Vue 3 + TypeScript + Vite + Element Plus |
| Terminal | xterm.js |
| Database | SQLite |
| Config | TOML at `~/.GoWebSSH/GoWebSSH.toml`, or `-ConfigDir` |
| Target | Linux ARM64 on ZTE U60Pro |

## Build

### Requirements

- Go 1.21+
- Node.js 18+
- npm
- `upx` optional, only for binary compression

### One-liner

Set `VERSION` to the release identifier you want embedded in the binary.

```bash
cd webssh && npm install && npm run build \
  && rsync -a --delete dist/ ../gossh/webroot/ \
  && cd ../gossh \
  && VERSION=dev \
  && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
     go build -ldflags="-s -w -X main.version=${VERSION}" -o webssh \
  && upx --best --lzma webssh
```

### Step By Step

```bash
# 1. Build the frontend
cd webssh
npm install
npm run build

# 2. Copy frontend assets into the backend webroot
rsync -a --delete dist/ ../gossh/webroot/

# 3. Cross-compile the backend for ARM64 Linux
cd ../gossh
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
  go build -ldflags="-s -w -X main.version=dev" -o webssh

# 4. Optional: compress the binary
upx --best --lzma webssh
```

## Deploy

Copy the compiled binary to the U60Pro and run it from its working directory.

```bash
scp gossh/webssh root@<router-ip>:/data/kano_plugins/webssh/

ssh root@<router-ip>
cd /data/kano_plugins/webssh
./webssh
```

The service listens on `:8899` by default. Open the web UI in a browser and complete the first-run setup wizard.

SQLite data is stored relative to the process working directory unless configured otherwise.

## Frontend Development

```bash
cd webssh
npm install
npm run dev
```

The Vite dev server starts at `http://127.0.0.1:3000/` by default.

Useful frontend commands:

```bash
npm run type-check
npm run build
npm run preview
```

## Backend Development

```bash
cd gossh
go build -o webssh
./webssh
```

For local full-stack testing, build the frontend first and sync `webssh/dist/` into `gossh/webroot/`.

## Repository Layout

```text
gossh/                Go backend
  main.go             Entry point and route registration
  app/config/         Configuration loading
  app/middleware/     HTTP middleware
  app/model/          SQLite models
  app/service/        SSH, SFTP, system, network, update, and device services
  webroot/            Compiled frontend static assets

webssh/               Vue 3 frontend
  src/views/          Main pages
  src/components/     Management panels
  src/stores/         Pinia state
  public/             PWA and static assets

docs/                 Project notes and router interface documentation
```

## Runtime Notes

- Run the binary from a persistent directory so SQLite data and generated files survive restarts.
- Place `cert.pem` and `key.key` in the working directory to enable HTTPS.
- Keep a backup of the SQLite database before replacing production binaries.
- The first account is created during initialization.

## License

[MIT](LICENSE)
