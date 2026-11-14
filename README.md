# core-backend

Go API for Goose. Handles sign-up and sign-in (email codes, passkeys, Truecaller), optional authenticator-app 2FA,
user profiles and usernames, and third-party account connections pushed in by the OAuth service.

**Stack:** Go 1.26 · chi · MongoDB · Redis · JWT · WebAuthn · TOTP · SMTP

## Getting started

Requirements: Go 1.26+, Docker, and [air](https://github.com/air-verse/air) for hot reload.

```sh
cp .env.example .env
make db-up        # MongoDB on :27017, Redis on :6379
make dev          # API on http://127.0.0.1:8080 with hot reload
curl localhost:8080/health
```

Emails go to `SMTP_HOST:SMTP_PORT` (default `localhost:1025`). For local testing run Mailpit and open
http://localhost:8025:

```sh
docker run -d -p 1025:1025 -p 8025:8025 axllent/mailpit
```

## Make targets

| Target | What it does |
|---|---|
| `make dev` | Run with hot reload (air) |
| `make run` | Run the server |
| `make build` | Build `bin/server` |
| `make test` | Run tests |
| `make lint` | `go vet` |
| `make fmt` / `make tidy` | `gofmt` / `go mod tidy` |
| `make db-up` / `db-down` / `db-logs` | Start, stop, tail local MongoDB and Redis |
| `make clean` | Remove build output |

## Configuration

Values are read from environment variables, then `.env`, then an optional `config.yaml` (in `.` or `./config`),
then built-in defaults. `.env.example` lists every option.

| Variable | Purpose |
|---|---|
| `ENV` | `dev` relaxes required secrets |
| `SERVER_HOST`, `SERVER_PORT`, `SERVER_TRUSTED_PROXIES` | Listen address; proxy CIDRs whose `X-Forwarded-For` is trusted |
| `MONGO_URI`, `MONGO_DATABASE` | MongoDB |
| `REDIS_URL`, `REDIS_PASSWORD`, `REDIS_DB` | Redis (email codes) |
| `JWT_SECRET`, `JWT_TTL` | Session tokens |
| `SECURITY_ENCRYPTION_KEY` | Encrypts 2FA secrets at rest (`openssl rand -base64 32`) |
| `WEBAUTHN_RP_ID`, `WEBAUTHN_RP_ORIGINS` | Passkey relying party |
| `TRUECALLER_CLIENT_ID`, `TRUECALLER_BASE_URL` | Truecaller OAuth |
| `SMTP_*` | Outgoing email |
| `RATE_LIMIT_GLOBAL_PER_MINUTE`, `RATE_LIMIT_AUTH_PER_MINUTE` | Per-IP limits |
| `MOBILE_ANDROID_*`, `MOBILE_IOS_APP_IDS` | Mobile passkey origins and well-known files |
| `WEBHOOKS_OAUTH_SECRET` | HMAC secret shared with the OAuth service; empty disables the webhook |

`JWT_SECRET` and `SECURITY_ENCRYPTION_KEY` are required whenever `ENV` is not `dev`.

## API

Full reference: [`docs/openapi.json`](docs/openapi.json).

| Method | Path | Auth |
|---|---|---|
| `GET` | `/health` | – |
| `POST` | `/v1/auth/register`, `/v1/auth/login` | – |
| `POST` | `/v1/auth/2fa/verify` | MFA token |
| `POST` / `DELETE` | `/v1/auth/link` | Bearer |
| `POST` | `/v1/auth/2fa/setup`, `/enable`, `/disable` | Bearer |
| `GET` | `/v1/users/me` | Bearer |
| `PUT` | `/v1/users/me/profile` | Bearer |
| `GET` | `/v1/users/me/connections` | Bearer |
| `GET` | `/v1/usernames/suggestions`, `/v1/usernames/availability` | Bearer |
| `POST` | `/v1/webhooks/oauth` | HMAC signature |
| `GET` | `/.well-known/assetlinks.json`, `/.well-known/apple-app-site-association` | – |

Auth routes have a stricter rate limit than the rest; the webhook is exempt.

### Sign-in flow

1. `POST /v1/auth/register` or `/v1/auth/login` with `method` set to `email`, `passkey` or `truecaller`.
2. If 2FA is enabled, the response carries an `mfa_token`; finish with `POST /v1/auth/2fa/verify`.
3. If `user.profile_complete` is `false`, collect name, username and date of birth via `PUT /v1/users/me/profile`.

Signed-in users can attach or remove extra sign-in methods with `POST` / `DELETE /v1/auth/link`.

### Connections webhook

The OAuth service owns provider tokens and pushes profile changes to `POST /v1/webhooks/oauth`.

- Header: `X-Webhook-Signature: t=<unix>,v1=<hex HMAC-SHA256(WEBHOOKS_OAUTH_SECRET, "<t>.<raw body>")>`
- Events: `connection.connected`, `connection.updated`, `connection.disconnected`
- Providers: YouTube, Roblox, Instagram, X, Spotify, GitHub, Riot Games, LinkedIn, Reddit, PlayStation,
  custom domain, Crunchyroll, Amazon Prime, Xbox, PayPal, Steam, Epic Games

`GET /v1/users/me/connections` returns every provider with `connected: true/false`.

### Mobile passkeys

Set `MOBILE_ANDROID_PACKAGE`, `MOBILE_ANDROID_CERT_FINGERPRINTS` and `MOBILE_IOS_APP_IDS`. The server then accepts the
Android app origin and serves the two `/.well-known/` files, which must be reachable at
`https://<WEBAUTHN_RP_ID>/.well-known/...`.

## Project layout

```
cmd/server/          entry point: config, connections, wiring, start
internal/
  api/routes/        every route and its middleware
  api/handlers/      decode request, call service, write JSON
  api/middleware/    JWT auth, client IP, rate limits, request logs
  auth/              sign-in flows, JWT, email codes, passkeys, Truecaller, 2FA, linking
  user/              user model, Mongo store, username and profile rules
  connection/        third-party connections from the OAuth webhook
  server/            HTTP server and graceful shutdown
pkg/                 app-agnostic helpers: config, logger, database, cache,
                     mailer, apperr, httpx, secret, webhook
templates/           email templates, embedded in the binary
docs/                OpenAPI spec
docker/              local MongoDB and Redis
```

Dependencies flow one way: `cmd` → `internal/api` → `internal/*` → `pkg`.
