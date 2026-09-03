# QRIS Dinamis

Open-source QRIS utility written in Go. Parse and validate QRIS strings, convert static QRIS into dynamic QRIS with a nominal and optional service fee, then generate a scannable QR image.

This project is a Go rewrite of [QRIS Dinamis](https://github.com/verssache/qris-dinamis), an MIT-licensed TypeScript project. Original copyright notice is retained in [LICENSE](LICENSE).

Web UI supports paste, upload, drag and drop, camera scanning, clipboard image scanning, light/dark mode, batch conversion, history, templates, CSV export, and offline conversion.

## Features

- QRIS TLV parsing and CRC16-CCITT validation
- Static QRIS to dynamic QRIS conversion
- Fixed or percentage service fee
- Batch conversion with CSV and ZIP QR export
- Local SQLite or Cloudflare D1 storage for admin transaction features
- Single Go HTTP server with bundled Vite frontend
- CLI and HTTP API

## Requirements

- Go 1.25+
- Bun 1.3+, used for frontend install, development, and production build

## Quick start (local)

```bash
git clone https://github.com/KevinAntonioWiyonoLauw/qris-dinamis-go.git
cd qris-dinamis-go
cp .env.example .env
go run ./cmd/qris-server
```

Open <http://localhost:8080>.

Default storage is local SQLite at `data/qris.db`. Admin login stays disabled until `ADMIN_PASS` is set to a password with at least 8 characters.

Never commit `.env`, database files, API tokens, or production credentials.

## CLI

```bash
go run ./cmd/qris-cli
```

## Cloudflare Pages deployment

Cloudflare Pages serves Vite output from `frontend/dist/` and deploys Functions in `functions/` on the same origin. The deployment includes QRIS API routes `/api/validate`, `/api/parse`, and `/api/convert`. Conversion also works offline in the browser.

Requirements: a Cloudflare account, Wrangler authentication, and Bun.

```bash
bun install --cwd frontend
bun run --cwd frontend build
bunx wrangler login
bunx wrangler pages deploy frontend/dist --project-name qris-dinamis-go
```

For Git-connected deploys, set these Cloudflare Pages build settings:

```text
Root directory: /
Build command: cd frontend && bun install --frozen-lockfile && bun run build
Build output directory: frontend/dist
```

`wrangler.toml` is included for the Pages output directory. No API token belongs in this file. Add production secrets in Cloudflare dashboard environment settings.

The original Go server remains available for self-hosting and for features that need Go libraries or persistent server storage, such as PDF/ZIP generation and admin transaction storage. Those Go-only routes are not part of the Pages Functions deployment.

## Frontend development

```bash
cd frontend
bun install
bun run dev
```

Production build writes generated assets to `frontend/dist/`:

```bash
bun run build
```

The optional Go server serves files from `WEB_DIR`, defaulting to `frontend/dist`, then falls back to legacy `web` paths. Go remains backend/API only; Vite owns frontend development and production assets.

## Tests

```bash
go test ./...
```

Core parity tests use `internal/qris/testdata/fixtures.json` to compare parsing, validation, and conversion output with the reference implementation.

## Configuration

Copy `.env.example` to `.env`. Important settings:

| Variable | Purpose | Default |
|---|---|---|
| `PORT` | HTTP port | `8080` |
| `STORAGE` | `sqlite` or `d1` | `sqlite` |
| `DATABASE_URL` | SQLite path or database URL | `data/qris.db` |
| `DATA_DIR` | Local data directory | `data` |
| `MIGRATIONS_DIR` | SQL migrations directory | `migrations` |
| `WEB_DIR` | Built frontend directory for optional Go server | `web` |
| `ADMIN_USER` | Admin username | `admin` |
| `ADMIN_PASS` | Admin password; min. 8 characters | empty |

Cloudflare D1 settings apply to the Go server deployment. Pages Functions currently do not require D1 for QRIS conversion.

## API

| Method | Path | Request / response |
|---|---|---|
| `POST` | `/api/validate` | `{"qris":"..."}` → validation result |
| `POST` | `/api/parse` | `{"qris":"..."}` → parsed QRIS data |
| `POST` | `/api/convert` | `{"qris":"...","amount":"25000","fee":{"type":"fixed","value":1000}}` → dynamic QRIS |
| `GET` | `/api/qr?data=<urlencoded>&size=280` | PNG QR image (Go server) |

Cloudflare Pages Functions:

| Method | Path | Status |
|---|---|---|
| `POST` | `/api/validate` | Available on Pages |
| `POST` | `/api/parse` | Available on Pages |
| `POST` | `/api/convert` | Available on Pages |

## Migrations

Migration files use paired names such as:

```text
000001_initial.up.sql
000001_initial.down.sql
```

The server applies unapplied `.up.sql` files in filename order. `.down.sql` files are rollback scripts and are not run automatically.

## Project layout

```text
internal/qris/       QRIS parser, validator, converter, and tests
cmd/qris-cli/         Interactive CLI
cmd/qris-server/      HTTP server, API, storage, and migrations
frontend/             Vite + React + Tailwind source and build output
functions/             Cloudflare Pages API Functions
web/                  Optional legacy Go-server build output
migrations/           SQL migrations
```

## Security

This project processes payment QR strings but does not process payment transactions. Review validation, authentication, rate limiting, HTTPS, and secret management before public deployment. Rotate any credential that was ever stored in a committed or shared `.env` file.

## License

MIT. See [LICENSE](LICENSE).
