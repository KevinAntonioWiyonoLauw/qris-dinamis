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
- Node.js 20+ and npm, only needed when rebuilding frontend assets

## Quick start

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

## Frontend development

```bash
cd frontend
npm install
npm run dev
```

Production build writes generated assets to `web/`:

```bash
npm run build
```

The Go server serves files from `WEB_DIR`, defaulting to `web`.

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
| `WEB_DIR` | Built frontend directory | `web` |
| `ADMIN_USER` | Admin username | `admin` |
| `ADMIN_PASS` | Admin password; min. 8 characters | empty |

Cloudflare D1 additionally needs `D1_ACCOUNT_ID`, `D1_DATABASE_ID`, and `CLOUDFLARE_API_TOKEN`, with `STORAGE=d1`.

## API

| Method | Path | Request / response |
|---|---|---|
| `POST` | `/api/validate` | `{"qris":"..."}` → validation result |
| `POST` | `/api/parse` | `{"qris":"..."}` → parsed QRIS data |
| `POST` | `/api/convert` | `{"qris":"...","amount":"25000","fee":{"type":"fixed","value":1000}}` → dynamic QRIS |
| `GET` | `/api/qr?data=<urlencoded>&size=280` | PNG QR image |

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
frontend/             React + Tailwind source
web/                  Generated frontend assets served by Go
migrations/           SQL migrations
```

## Security

This project processes payment QR strings but does not process payment transactions. Review validation, authentication, rate limiting, HTTPS, and secret management before public deployment. Rotate any credential that was ever stored in a committed or shared `.env` file.

## License

MIT. See [LICENSE](LICENSE).
