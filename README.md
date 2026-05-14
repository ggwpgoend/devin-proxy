# devin-proxy

Transparent reverse proxy for Devin API with automatic key rotation across multiple accounts.

## How it works

```
Your client → localhost:9090 → (auto-pick key) → api.devin.ai
                                    ↓
                              401/402/429?
                              rotate → retry
```

- **Proxy** listens on `:9090` — drop-in replacement for `https://api.devin.ai`
- **Admin panel** on `:9091` — manage keys, view stats, request logs
- **Round-robin** with priority: trial → free → paid
- **Auto-rotation** on 401 (unauthorized), 402 (quota), 429 (rate limit)
- **Cooldown** for exhausted keys (default 10 min), auto-reactivation
- **AES-256-GCM** encryption for stored keys
- **SQLite** database, zero external dependencies

## Quick start

### From binary

```bash
# Download or build the binary
./devin-proxy

# Or with custom ports:
./devin-proxy --proxy-port 9090 --admin-port 9091 --cooldown 10
```

### From source

```bash
git clone https://github.com/ggwpgoend/devin-proxy.git
cd devin-proxy
go build -o devin-proxy ./cmd/devin-proxy
./devin-proxy
```

### Windows

```bash
# Cross-compile:
GOOS=windows GOARCH=amd64 go build -o devin-proxy.exe ./cmd/devin-proxy

# Or use make:
make windows
```

## Usage

1. Start the proxy: `./devin-proxy`
2. Open admin panel: `http://localhost:9091`
3. Add your API keys (single or bulk import)
4. Point your client to `http://localhost:9090` instead of `https://api.devin.ai`

### Example with curl

```bash
# Instead of:
curl -H "Authorization: Bearer YOUR_KEY" https://api.devin.ai/v1/sessions

# Use:
curl http://localhost:9090/v1/sessions
# The proxy adds the Authorization header automatically
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--proxy-port` | 9090 | Port for the reverse proxy |
| `--admin-port` | 9091 | Port for the admin dashboard |
| `--data-dir` | `.` | Directory for database and master key |
| `--cooldown` | 10 | Cooldown duration (minutes) for exhausted keys |

## Admin panel features

- **Dashboard** with KPI cards (total/active/cooldown/revoked keys, request stats)
- **Add single key** with label and plan type
- **Bulk import** — paste multiple keys (one per line or comma-separated)
- **Enable/disable/delete** keys
- **Request log** — last 50 proxied requests with status, latency, errors
- **Auto-refresh** every 10 seconds

## Architecture

```
cmd/devin-proxy/main.go    — entrypoint, flag parsing, server wiring
internal/
  crypto/crypto.go         — AES-256-GCM encryption for keys at rest
  store/store.go           — SQLite with embedded migrations
  keypool/pool.go          — key pool: CRUD, round-robin pick, cooldown, stats
  proxy/proxy.go           — HTTP reverse proxy with retry & rotation
  admin/admin.go           — admin dashboard routes (chi)
  admin/templates.go       — embedded HTML/CSS dashboard
```

## Tests

```bash
go test ./... -v
```
