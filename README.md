# QRIS Dinamis

Open-source QRIS utility. Parse and validate QRIS strings, convert static QRIS into dynamic QRIS with a nominal and optional service fee, then generate a scannable QR image.

Live demo: <https://qris.kevinio.my.id>

Web UI supports paste, upload, camera scanning, clipboard image scanning, light/dark mode, batch conversion, history, templates, CSV export, and offline conversion.

## Features

- QRIS TLV parsing and CRC16-CCITT validation
- Static QRIS to dynamic QRIS conversion (with CRC recompute)
- Fixed or percentage service fee
- Batch conversion with CSV export
- SVG QR generation
- Offline mode via client-side engine (`frontend/src/lib/qris.ts`)
- Cloudflare Pages deployment with Functions API

## Stack

- React + TypeScript + Vite + Tailwind (frontend)
- Bun (install, dev, build)
- Cloudflare Pages Functions (`functions/`) for API routes

## Development

```bash
git clone https://github.com/KevinAntonioWiyonoLauw/qris-dinamis.git
cd qris-dinamis/frontend
bun install
bun run dev
```

Production build:

```bash
bun run build   # outputs frontend/dist
```

## Cloudflare Pages deployment

Pages serves Vite output from `frontend/dist/` and deploys Functions in `functions/` on the same origin. API routes: `/api/validate`, `/api/parse`, `/api/convert`, `/api/qr` (SVG). Conversion also works fully offline in the browser.

Requirements: a Cloudflare account, Wrangler authentication, and Bun.

```bash
bun install            # install root deps (functions)
cd frontend && bun install && bun run build
bunx wrangler login
bunx wrangler pages deploy frontend/dist --project-name qris-dinamis-go
```

For Git-connected deploys, the build configuration lives in the Cloudflare Pages dashboard settings (project qris-dinamis-go):

```text
Root directory: /
Build command: bun install && cd frontend && bun install && bun run build
Build output directory: frontend/dist
```

`wrangler.toml` holds the Pages project name and compatibility date. No API token belongs in this file. Add secrets in Cloudflare dashboard environment settings.

## API

| Method | Path | Request / response |
|---|---|---|
| `POST` | `/api/validate` | `{"qris":"..."}` → validation result |
| `POST` | `/api/parse` | `{"qris":"..."}` → parsed QRIS data |
| `POST` | `/api/convert` | `{"qris":"...","amount":"25000","fee":{"type":"fixed","value":1000}}` → dynamic QRIS |
| `GET` | `/api/qr?data=<urlencoded>&size=280` | SVG QR image |

## Project layout

```text
frontend/src/lib/qris.ts   QRIS parse/validate/convert engine (shared by UI and Functions)
frontend/                  Vite + React + Tailwind source
functions/                 Cloudflare Pages API Functions
```

## Security

This project processes payment QR strings but does not process payment transactions. Review validation, authentication, rate limiting, HTTPS, and secret management before public deployment. Rotate any credential that was ever stored in a committed or shared `.env` file.

## License

MIT. See [LICENSE](LICENSE).