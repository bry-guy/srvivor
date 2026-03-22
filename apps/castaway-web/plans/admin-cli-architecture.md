# Admin CLI for Castaway — Idea Doc

## Context

The castaway ecosystem (castaway-web API, castaway-discord-bot, legacy CLI) is deployed via ArgoCD to a self-hosted k3s cluster on Proxmox VMs, networked through Tailscale. The web API has service-to-service bearer token auth but no human-facing auth, no RBAC, and no endpoints for managing activities, occurrences, groups, or bonus ledger writes. The goal is to build an admin CLI ("game master" tool) that authenticates human operators via OAuth, enforces RBAC, and exposes the full write surface of castaway-web.

## Solution Shape

Three work streams that compose into one system:

1. **Self-hosted OIDC provider** (Zitadel) — issues JWTs to CLI users, stores roles
2. **castaway-web auth + API expansion** — validates OIDC JWTs, adds missing endpoints, enforces RBAC
3. **`castaway-admin` CLI** — Cobra-based Go CLI, authenticates via OAuth2 Device Grant, calls castaway-web

---

## 1. OIDC Provider: Zitadel

### Why Zitadel over alternatives

| Provider | Fit | Issue |
|----------|-----|-------|
| **Zitadel** | Best | Go-native, single binary, Device Grant support, ~100-150MB RAM, PostgreSQL backend, Apache 2.0 |
| Authelia | Poor | Reverse-proxy SSO only, can't issue tokens for CLI→API flows |
| Authentik | Heavy | Python/Django + Redis + PG, ~500MB+ RAM |
| Keycloak | Heavy | Java, ~500MB+ RAM |
| Ory Hydra | OK | Lightweight but requires deploying Kratos separately for identity |
| Kanidm | Thin ecosystem | No Go SDK, limited Device Grant support |

### Deployment

- Runs on the same k3s cluster, deployed via ArgoCD + Kustomize
- Uses the **existing PostgreSQL cluster** (separate `zitadel` database alongside `castaway`)
- Ingress at `zitadel.castaway.internal` (reachable over Tailscale)
- Secrets (masterkey, DB password) in 1Password vault `castaway`, injected via fnox
- TLS via Tailscale cert provisioning or self-signed CA (tiny user base)

### Zitadel project config

- Project: `castaway`
- Roles: `admin`, `viewer`
- Application: `castaway-admin-cli` (type: Native, Device Code grant enabled, no client secret)
- Initial admin user created manually
- Optional future: machine user for discord bot (migrate away from static bearer tokens)

---

## 2. CLI Authentication: OAuth2 Device Authorization Grant (RFC 8628)

### Why Device Grant

- Works over SSH and headless environments (no local HTTP server)
- No port conflicts (unlike authorization code with localhost redirect)
- User can authorize from any browser (phone via Tailscale works)
- Industry standard for CLIs (GitHub CLI, Azure CLI, etc.)

### Flow

1. `castaway-admin auth login`
2. CLI calls Zitadel device authorization endpoint → receives `device_code` + `user_code`
3. CLI prints: `Open https://zitadel.castaway.internal/device and enter code: ABCD-1234`
4. User opens browser, authenticates with Zitadel, enters code
5. CLI polls token endpoint → receives access token (JWT) + refresh token
6. Tokens stored at `~/.config/castaway/credentials.json` (0600 permissions)
7. Subsequent commands use access token; auto-refresh when expired
8. If refresh token expired → CLI prompts re-login

### Config

```yaml
# ~/.config/castaway/config.yaml
server_url: https://castaway.internal
oidc_issuer: https://zitadel.castaway.internal
client_id: "castaway-admin-cli"
default_instance: "<uuid>"  # optional convenience
```

---

## 3. castaway-web Changes

### 3a. Dual Auth Middleware

Both auth paths coexist permanently — service tokens for the bot, OIDC JWTs for humans.

Extend `internal/httpapi/auth.go`:
- Try OIDC JWT validation first (via JWKS from Zitadel, cached with `github.com/MicahParks/keyfunc/v3`)
- Fall back to existing static bearer token check
- Both paths produce a unified `CallerIdentity`:

```go
type CallerIdentity struct {
    Subject string   // OIDC sub or "castaway-discord-bot"
    Kind    string   // "user" or "service"
    Roles   []string // ["admin"] from Zitadel claims, or implicit for service
    Name    string   // human-readable
}
```

New env vars: `OIDC_ENABLED`, `OIDC_ISSUER_URL`, `OIDC_AUDIENCE`

### 3b. RBAC

Roles live in **OIDC claims** (Zitadel project roles), not in the castaway database. This avoids building user management.

- **Write endpoints** (POST/PUT/DELETE): require `admin` role or `service` kind
- **Read endpoints** (GET): any authenticated caller
- **Health**: no auth (unchanged)

### 3c. New API Endpoints

The database schema already has tables for activities, occurrences, groups, and bonus entries (migrations 006/007). The sqlc queries exist. Missing: HTTP handlers.

**Activities**: `POST/GET /instances/:id/activities`, `GET /instances/:id/activities/:aid`
**Occurrences**: `POST/GET /instances/:id/activities/:aid/occurrences`, resolve endpoint
**Groups**: `POST/GET /instances/:id/groups`, membership management
**Assignments**: group and participant assignments to activities
**Bonus ledger writes**: admin-only endpoint for creating ledger entries

All follow existing patterns in `server.go`.

---

## 4. `apps/castaway-admin` CLI

### Structure

```
apps/castaway-admin/
  go.mod
  main.go
  cmd/
    root.go         # config loading, global flags
    auth.go         # login, status, logout
    instance.go     # list, get, create
    activity.go     # list, get, create
    occurrence.go   # list, get, create, resolve
    group.go        # list, create, add-member
    draft.go        # get, submit
    leaderboard.go  # get
    bonus.go        # ledger
  internal/
    client/         # HTTP client (modeled on discord bot's castaway/client.go)
    config/         # XDG config loading
    output/         # table + JSON formatters
```

### Command Tree

```
castaway-admin auth login|status|logout
castaway-admin instance list|get|create
castaway-admin activity list|get|create
castaway-admin occurrence list|get|create|resolve
castaway-admin group list|create|add-member|members
castaway-admin draft get|submit
castaway-admin leaderboard get
castaway-admin bonus ledger
```

### Global Flags

`--server-url`, `--output table|json`, `--instance <uuid>`, `--config <path>`

### Dependencies

`spf13/cobra`, `golang.org/x/oauth2`, `gopkg.in/yaml.v3` — all already in the monorepo ecosystem.

---

## 5. Infrastructure (Terraform + k8s)

### New Kustomize resources

```
deploy/base/zitadel/           # Deployment, Service, ConfigMap
deploy/environments/home-k3s/  # Ingress, patches, placement, secrets
```

Zitadel added to ArgoCD alongside castaway-web and castaway-discord-bot.

### What stays in the infra repo (Terraform/Proxmox)

Nothing new needed — the existing VM substrate and k3s cluster are sufficient. Zitadel runs as another workload on the same cluster.

---

## 6. Phased Roadmap

### Phase 1: API Endpoints (no auth changes)
- Add activity/occurrence/group/bonus CRUD handlers to castaway-web
- Protected by existing bearer token middleware
- Integration tests, TypeSpec/OpenAPI updates
- **No new infrastructure**

### Phase 2: CLI Scaffold with Bearer Token Auth
- Create `apps/castaway-admin/` with Cobra commands
- HTTP client based on discord bot's `internal/castaway/client.go`
- Core commands: `instance list`, `activity create`, `occurrence create/resolve`
- Uses static bearer token (same as discord bot) as temporary auth
- **No Zitadel yet**

### Phase 3: Deploy Zitadel
- Kustomize base + home-k3s overlay
- 1Password secrets + fnox profile
- ArgoCD deployment
- Configure project, roles, CLI application, initial admin user
- Validate Device Grant flow manually
- **Zitadel running, nothing uses it yet**

### Phase 4: OIDC Integration
- Dual auth middleware in castaway-web (OIDC JWT + legacy bearer)
- RBAC enforcement on write endpoints
- CLI `auth login` with Device Grant, token storage/refresh
- **Both auth paths active. Bot unchanged. CLI uses OIDC.**

### Phase 5: Polish
- Shell completions, colored output, `--output json`
- Default instance, actionable error messages
- Optional: migrate discord bot to Zitadel machine user

---

## Key Files

| File | Role |
|------|------|
| `apps/castaway-web/internal/httpapi/auth.go` | Auth middleware to extend |
| `apps/castaway-web/internal/httpapi/server.go` | Router + handlers for new endpoints |
| `apps/castaway-web/internal/gameplay/service.go` | Business logic the new endpoints call |
| `apps/castaway-discord-bot/internal/castaway/client.go` | Reference HTTP client pattern |
| `deploy/environments/home-k3s/kustomization.yaml` | Where Zitadel resources get added |

## Verification

- Phase 1: Hurl regression tests for new endpoints (extend `apps/castaway-web/hurl/`)
- Phase 2: `castaway-admin instance list` returns data against local docker-compose stack
- Phase 3: `curl` Device Grant flow against Zitadel, receive valid JWT
- Phase 4: `castaway-admin auth login` → `castaway-admin activity create` end-to-end
- Phase 5: `castaway-admin --output json leaderboard get <id> | jq`
