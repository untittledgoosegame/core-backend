# core-backend

Go API backed by MongoDB and Redis. Sign-in with email codes, passkeys or Truecaller, optional authenticator-app 2FA, then profile setup.

## Quick start

```sh
cp .env.example .env
make db-up      # MongoDB + Redis + Mailpit (emails show up at http://localhost:8025)
make dev        # hot reload with air
```

`make help` lists every target. API reference: [`docs/openapi.json`](docs/openapi.json).

## Folder structure

```
cmd/server/            entry point: load config, connect, wire everything, start the server
internal/              application code (not importable by other modules)
  api/
    routes/            every URL and the middleware applied to it, in one file
    handlers/          HTTP handlers: decode request, call a service, write JSON
    middleware/        auth (JWT), client IP, rate limiting, request logging
  auth/                sign-in logic, one file per concern
    service.go         Register / Login entry points and session issuing
    email.go           email code flows
    passkey.go         WebAuthn flows and mobile passkey origins
    truecaller.go      Truecaller OAuth client and flows
    twofactor.go       TOTP setup, enable, disable, verify
    link.go            link / unlink extra sign-in methods
    otp.go             email codes in Redis (hashed, expiring, attempt-limited)
    challenges.go      passkey and 2FA challenges (Mongo, TTL indexed)
    token.go           JWT issue / parse
  user/                user model, Mongo store, username + profile rules
  connection/          third-party connections (connected flag + profile) fed by the OAuth webhook
  server/              HTTP server start / graceful shutdown
pkg/                   reusable building blocks with no app logic
  config/              Viper config (.env, config.yaml, environment variables)
  logger/              charmbracelet/log setup (level, text or json)
  database/            MongoDB connection
  cache/               Redis connection
  mailer/              SMTP sender + template rendering
  apperr/              errors that carry an HTTP status
  httpx/               JSON decode / respond / error helpers
  secret/              AES-256-GCM encryption for secrets at rest
  webhook/             HMAC webhook signing and verification
templates/             email templates (embedded into the binary)
docs/                  OpenAPI spec
docker/                local MongoDB, Redis + Mailpit
```

Dependencies point one way: `cmd` → `internal/api` → `internal/auth`, `internal/user` → `pkg`.

## Configuration

Settings are read in this order, first match wins: environment variables, `.env`, `config.yaml`, built-in defaults.
See `.env.example` (or `config.example.yaml`) for every option.

Required outside `ENV=dev`: `JWT_SECRET` and `SECURITY_ENCRYPTION_KEY` (`openssl rand -base64 32`).

## Auth flow

1. `POST /v1/auth/register` or `/v1/auth/login` with `method` = `email`, `passkey` or `truecaller`.
2. If 2FA is on, login returns `mfa_token`; finish with `POST /v1/auth/2fa/verify`.
3. If `user.profile_complete` is false, collect name, username, date of birth (`PUT /v1/users/me/profile`).

Signed-in users can add or remove methods with `POST` / `DELETE /v1/auth/link`.

## Connections (OAuth service webhook)

A separate OAuth service connects users to YouTube, Roblox, Instagram, X, Spotify, GitHub, Riot Games, LinkedIn, Reddit,
PlayStation, custom domains, Crunchyroll, Amazon Prime, Xbox, PayPal, Steam and Epic Games. It keeps the tokens and
sends profile changes here:

- **Webhook URL:** `POST /v1/webhooks/oauth`
- **Signature:** `X-Webhook-Signature: t=<unix>,v1=<hex HMAC-SHA256(WEBHOOKS_OAUTH_SECRET, "<t>.<raw body>")>`
- **Events:** `connection.connected`, `connection.updated`, `connection.disconnected`, each with a unique `id`,
  `occurred_at`, `user_id`, `provider` and (except disconnect) the `account` profile.

Stored in the `connections` collection, one document per user and provider. Apps read
`GET /v1/users/me/connections`, which lists all 17 providers with `connected: true/false`.
Full payload and response details are in `docs/openapi.json` (`webhooks` section).

## Mobile passkeys

Set `MOBILE_ANDROID_PACKAGE`, `MOBILE_ANDROID_CERT_FINGERPRINTS` and `MOBILE_IOS_APP_IDS`. The server then accepts
the Android app's origin and serves `/.well-known/assetlinks.json` and `/.well-known/apple-app-site-association`.
Those two files must be reachable at `https://<WEBAUTHN_RP_ID>/.well-known/...`, so route them to this service.
