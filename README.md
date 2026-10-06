# Yomi — Citizen Data Consent Ledger

Yomi is a distributed consent-management and audit platform for India's digital
ecosystem. Institutions (banks, hospitals, telecom providers, government
departments) must request explicit citizen approval before accessing personal
data. Every request, approval, rejection, and revocation is cryptographically
signed and recorded in an immutable, hash-chained audit ledger — giving citizens
full transparency and control over how their data is used.

```
┌──────────────┐   /api/*    ┌─────────────┐   JWT + rate limit + CORS
│   Frontend   │ ──────────► │   Gateway   │──────────────────────────────┐
│ React + Vite │             │  (8080)     │                              │
└──────────────┘             └─────────────┘                              │
                                   │  proxy                                ▼
              ┌────────────────────┼─────────────────────┐         ┌──────────────┐
              ▼                    ▼                     ▼         │  Institution  │
        ┌───────────┐       ┌───────────┐         ┌───────────┐   │    (8084)     │
        │   Auth    │       │  Consent  │         │   Audit   │   └──────────────┘
        │  (8081)   │       │  (8082)   │         │   (8083)  │
        └───────────┘       └─────┬─────┘         └─────┬─────┘
              │                   │  publish            │ consume
              ▼                   ▼                     ▼
         ┌────────┐         ┌──────────┐         ┌──────────┐
         │ Redis  │         │  Kafka   │────────►│PostgreSQL│
         │  (7)   │         │ (KRaft)  │         │  (15)    │
         └────────┘         └──────────┘         └──────────┘
```

## Architecture

Five Go microservices, event-driven through Kafka, each owning its own
PostgreSQL schema (no cross-service database access):

| Service     | Port | Responsibility                                                                 |
|-------------|------|--------------------------------------------------------------------------------|
| Gateway     | 8080 | Single entry point: JWT validation, token-bucket rate limiting, CORS, request-ID logging, reverse-proxying |
| Auth        | 8081 | Citizen registration (bcrypt), RS256 JWT access tokens (15 min), refresh tokens (7 days, Redis), logout |
| Consent     | 8082 | Institution data-access requests, citizen approve/reject/revoke, HMAC-SHA256 authorization tokens, publishes events to Kafka |
| Audit       | 8083 | Consumes `consent-events`, appends to an append-only hash-chained ledger, verifies chain integrity |
| Institution | 8084 | Institution CRUD, API-key issue/rotate/revoke, internal key-authentication for the consent service |

Shared infrastructure: PostgreSQL 15 (four databases, one per service),
Redis 7 (refresh tokens, rate-limit state), Kafka in KRaft mode (no Zookeeper).

### Security model

- Passwords: bcrypt (`golang.org/x/crypto/bcrypt`), never stored in plaintext.
- Access tokens: RS256 JWTs, 15-minute expiry, held **in React state only**
  (never localStorage/sessionStorage).
- Refresh tokens: cryptographically random, stored hashed in Redis, delivered
  as an **httpOnly cookie**; rotated on every refresh.
- Consent authorization tokens: HMAC-SHA256 signed payloads containing request
  ID, citizen ID, expiry, and scope.
- Institution identity on `POST /consent/request` is derived from the
  `X-API-Key` header and verified service-to-service — a client can never
  spoof another institution's ID.
- Audit ledger: append-only table (no UPDATE/DELETE in the codebase), every
  entry carries `prev_hash` + `entry_hash`; the audit service recomputes the
  chain on demand (`GET /audit/verify`).

## Quick start (Docker)

```bash
# 1. Generate JWT signing keys (one-time)
./scripts/gen-keys.sh

# 2. Create your .env (edit secrets before any real deployment)
cp .env.example .env

# 3. Build and start the full stack
docker compose up --build

# 4. Open the app
open http://localhost:8088
```

The stack comes up in dependency order (Postgres → Redis → Kafka → services →
gateway → frontend) with health checks on the stateful services. Migrations
run automatically on service startup from embedded SQL files.

### Local development (without Docker)

```bash
# Infrastructure via Docker only
docker compose up -d postgres redis kafka

# Backend (each service in its own terminal)
cd backend
cp ../.env .env        # godotenv loads it
go run ./cmd/auth
go run ./cmd/consent
go run ./cmd/audit
go run ./cmd/institution
go run ./cmd/gateway

# Frontend
cd frontend
npm install
npm run dev            # http://localhost:5173, proxies /api -> :8080
```

## Trying a full consent flow

```bash
BASE=http://localhost:8080

# 1. Register a citizen
curl -s -X POST $BASE/auth/register -H 'Content-Type: application/json' -d '{
  "fullName":"Asha Verma","aadhaarId":"123456789012",
  "email":"asha@example.com","phone":"9876543210","password":"correct-horse"}' | jq

# 2. Login (capture access token + httpOnly refresh cookie)
curl -s -c cookies.txt -X POST $BASE/auth/login -H 'Content-Type: application/json' \
  -d '{"aadhaarId":"123456789012","password":"correct-horse"}' | jq -r .data.accessToken > token.txt

# 3. Register an institution (returns one-time API key)
curl -s -X POST $BASE/institution/register -H 'Content-Type: application/json' \
  -d '{"name":"Apna Bank","sector":"banking"}' | jq

# 4. Institution requests data access
curl -s -X POST $BASE/consent/request -H "X-API-Key: yomi_<key>" \
  -H 'Content-Type: application/json' -d '{
    "citizenId":"<citizen-uuid>",
    "dataScope":["account.balance","account.transactions"],
    "purpose":"KYC verification for loan application",
    "expiresAt":"2026-12-31T23:59:59Z"}' | jq

# 5. Citizen approves
curl -s -X POST $BASE/consent/approve/<request-id> \
  -H "Authorization: Bearer $(cat token.txt)" | jq

# 6. Verify the ledger chain
curl -s $BASE/audit/verify -H "Authorization: Bearer $(cat token.txt)" | jq
```

## Environment variables

| Variable                 | Services          | Default                          | Description                                   |
|--------------------------|-------------------|----------------------------------|-----------------------------------------------|
| `PORT`                   | all               | per-service (8080–8084)          | HTTP listen port                              |
| `DB_URL`                 | all               | —                                | Postgres DSN for the service's database       |
| `REDIS_ADDR`             | auth, consent     | `localhost:6379`                 | Redis host:port                               |
| `REDIS_PASSWORD`         | auth, consent     | —                                | Redis auth password                           |
| `REDIS_DB`               | auth, consent     | `0`                              | Redis logical DB                              |
| `KAFKA_BROKERS`          | consent, audit    | `localhost:9092`                 | Comma-separated broker addresses              |
| `JWT_PRIVATE_KEY_PATH`   | auth              | —                                | Path to RSA private key PEM (signing)         |
| `JWT_PUBLIC_KEY_PATH`    | all except auth   | —                                | Path to RSA public key PEM (verification)     |
| `ACCESS_TOKEN_TTL`       | auth              | `15m`                            | Access-token lifetime                         |
| `REFRESH_TOKEN_TTL`      | auth              | `168h`                           | Refresh-token lifetime (Redis TTL)            |
| `REFRESH_COOKIE_NAME`    | auth              | `yomi_refresh`                   | httpOnly refresh cookie name                  |
| `REFRESH_COOKIE_DOMAIN`  | auth              | —                                | Cookie Domain attribute (optional)            |
| `CONSENT_TOKEN_SECRET`   | consent           | dev-only                         | HMAC secret for authorization tokens (≥32 chars in prod) |
| `CONSENT_TOKEN_TTL`      | consent           | `24h`                            | Authorization-token lifetime                  |
| `INTERNAL_SERVICE_SECRET`| consent, institution | —                             | Shared secret for service-to-service calls    |
| `SERVICE_AUTH_URL`       | gateway           | `http://localhost:8081`          | Downstream auth base URL                      |
| `SERVICE_CONSENT_URL`    | gateway           | `http://localhost:8082`          | Downstream consent base URL                   |
| `SERVICE_AUDIT_URL`      | gateway           | `http://localhost:8083`          | Downstream audit base URL                     |
| `SERVICE_INSTITUTION_URL`| gateway, consent | `http://localhost:8084`          | Downstream institution base URL               |
| `RATE_LIMIT_RPS`         | gateway           | `20`                             | Token-bucket refill rate per IP               |
| `RATE_LIMIT_BURST`       | gateway           | `40`                             | Token-bucket burst capacity                   |
| `CORS_ORIGINS`           | gateway           | `http://localhost:5173,http://localhost:8080` | Allowed origins (comma-separated) |

## API reference

All responses are JSON envelopes `{"data": ...}`; all errors are
`{"error": "...", "code": "ERROR_CODE"}`.

### Auth service (via gateway `/api/auth/...`)

| Method | Path                | Auth        | Body / Params                                    | Success          |
|--------|---------------------|-------------|--------------------------------------------------|------------------|
| POST   | `/auth/register`    | none        | `{fullName, aadhaarId, email, phone?, password}` | 201 `Citizen`    |
| POST   | `/auth/login`       | none        | `{aadhaarId, password}`                          | 200 `{accessToken, citizen}` + refresh cookie |
| POST   | `/auth/refresh`     | refresh cookie | —                                              | 200 `{accessToken, citizen}` + new cookie |
| POST   | `/auth/logout`      | refresh cookie | —                                              | 200 `{status: "logged_out"}` |

### Consent service (via gateway `/api/consent/...`)

| Method | Path                     | Auth                     | Body / Params                                | Success            |
|--------|--------------------------|--------------------------|----------------------------------------------|--------------------|
| POST   | `/consent/request`       | `X-API-Key` (institution) | `{citizenId, dataScope[], purpose, expiresAt, institutionId?}` | 201 `ConsentRequest` |
| GET    | `/consent/pending/:citizenId` | citizen JWT          | —                                            | 200 `ConsentRequest[]` |
| POST   | `/consent/approve/:requestId` | citizen JWT          | —                                            | 200 `ConsentRequest` (auth token issued) |
| POST   | `/consent/reject/:requestId`  | citizen JWT          | —                                            | 200 `ConsentRequest` |
| DELETE | `/consent/revoke/:requestId`  | citizen JWT          | —                                            | 200 `ConsentRequest` |
| GET    | `/consent/history/:citizenId` | citizen JWT          | —                                            | 200 `ConsentRequest[]` |

### Audit service (via gateway `/api/audit/...`)

| Method | Path                       | Auth       | Params        | Success                    |
|--------|----------------------------|------------|---------------|----------------------------|
| GET    | `/audit/citizen/:citizenId` | JWT        | —             | 200 `AuditEntry[]`         |
| GET    | `/audit/institution/:institutionId` | JWT | —           | 200 `AuditEntry[]`         |
| GET    | `/audit/event/:eventId`     | JWT        | —             | 200 `AuditEntry`           |
| GET    | `/audit/verify`             | JWT        | —             | 200 `{intact, entriesChecked, chain[]}` |

### Institution service (via gateway `/api/institution/...`)

| Method | Path                         | Auth       | Body / Params             | Success                         |
|--------|------------------------------|------------|---------------------------|---------------------------------|
| POST   | `/institution/register`      | none       | `{name, sector}`          | 201 `{data: Institution, apiKey}` |
| POST   | `/institution/authenticate`  | internal secret header | `{apiKey}` | 200 `Institution`      |
| GET    | `/institution/:id`           | JWT        | —                         | 200 `Institution`               |
| POST   | `/institution/:id/rotate-key`| JWT        | —                         | 200 `{data: Institution, apiKey}` |

## Kafka events

Topic `consent-events` (3 partitions, KRaft):

```json
{
  "event_id": "uuid",
  "event_type": "consent_requested | consent_approved | consent_rejected | consent_revoked | data_accessed",
  "citizen_id": "uuid",
  "institution_id": "uuid",
  "request_id": "uuid",
  "timestamp": "ISO8601",
  "metadata": {}
}
```

## Frontend

React 18 + TypeScript (strict) + Vite + Tailwind v3 + React Router v6.
State via React Context + `useReducer`; HTTP via Axios with a single-flight
JWT auto-refresh interceptor. The audit page renders the ledger as a visual
hash chain — each row shows `prev_hash → entry_hash` with a connecting rail,
and a server-side recomputation marks any tampered entry.

| Page                | Route                 | Notes                                              |
|---------------------|-----------------------|----------------------------------------------------|
| Login               | `/login`              | Aadhaar + password                                 |
| Register            | `/register`           | Citizen signup                                     |
| Dashboard           | `/dashboard`          | Stats, recent ledger activity, quick actions       |
| Pending requests    | `/consent/pending`    | Approve/reject, auto-polling every 8s              |
| Consent history     | `/consent/history`    | Sortable, filterable, paginated table              |
| Audit ledger        | `/audit`              | Hash-chain visualization + integrity verification  |
| Profile             | `/profile`            | Identity, institutions, active consent tokens + revoke |

## Repository layout

```
backend/
  cmd/{gateway,auth,consent,audit,institution}/main.go
  internal/
    {auth,consent,audit,institution}/   service logic + migrations/
    middleware/    JWT, rate limiter, logging, CORS
    kafka/         producer + consumer wrappers
    db/            pgx pool + embedded migration runner
    cache/         Redis client
    models/        shared structs
    config/        env-based config
    instclient/    consent -> institution HTTP client
  Dockerfile       multi-stage (ARG SERVICE)
frontend/
  src/
    api/           axios instance + per-service modules
    components/    layout, consent, audit, common
    context/       AuthContext, ConsentContext
    hooks/         useAuth, useConsent, useAudit, usePoll
    pages/         Login, Register, Dashboard, PendingConsents,
                   ConsentHistory, AuditLog, Profile
    router/        AppRouter, ProtectedRoute
    types/         domain interfaces
    utils/         formatDate, truncateHash, maskAadhaar, ...
    styles/        Tailwind entry + design tokens
  Dockerfile       node build -> nginx
docker-compose.yml
.env / .env.example
scripts/gen-keys.sh, init-databases.sh
```

## Linting & verification

```bash
cd backend
golangci-lint run      # govet, staticcheck, errcheck, unused, ineffassign, gosimple
go vet ./...
go build ./...
go test ./...           # unit tests: audit hashing, consent tokens, auth, rate limiter

cd frontend
npm run build          # tsc -b (strict) + vite build
```

The Go unit tests live alongside their packages and cover the core
cryptographic and security logic without needing infrastructure:

| Package      | Coverage                                                                  |
|--------------|---------------------------------------------------------------------------|
| `internal/audit`       | `hashEntry` determinism, microsecond timestamp truncation, field-sensitivity, `FromEvent` mapping & idempotency |
| `internal/consent`     | HMAC-SHA256 authorization token signing, claims (request/citizen/expiry/scope), secret rotation |
| `internal/auth`        | bcrypt round-trip & salt, refresh-token hashing, opaque token generation  |
| `internal/middleware`  | token-bucket rate limiter (burst, per-key isolation, refill)              |

For a full end-to-end check against a running stack, exercise the API
lifecycle described in *Trying a full consent flow* — registration → login →
institution → request → approve → reject → revoke → `GET /audit/verify`
(which must report an intact, genesis-anchored chain).

## Notes

- The gateway rate-limits per client IP with a token bucket; `Retry-After`
  is set on 429 responses.
- Request IDs are generated at the gateway and propagated to downstream
  services via the `X-Request-ID` header and structured `log/slog` output.
- `GET /audit/verify` recomputes every entry hash from the genesis anchor and
  reports `intact` plus per-entry validity — the frontend renders any breach.
- Rotating an institution API key immediately invalidates the previous key;
  revoking an institution deactivates its key entirely.
