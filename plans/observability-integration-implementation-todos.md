# Observability integration implementation todos

Status: planning

This is the implementation checklist for `plans/observability-integration-planning.md`.

## Phase 0 — Baseline and ownership

Goal: make sure the current system state and repo ownership are explicit before adding instrumentation.

Status: done 2026-05-20, with one local-auth caveat: `mise run selfhost:cloudflare-dns:plan` still times out on fnox/1Password auth, but live DNS already resolves `home.bry-guy.net` to the appliance MagicDNS target and HTTPS smoke tests pass.

### Infra repo baseline

- [x] Confirm `home.bry-guy.net` DNS has been applied after 1Password/fnox auth is available.
  - [ ] `mise run selfhost:cloudflare-dns:apply` — skipped/blocked by local fnox/1Password auth timeout; live DNS is already correct.
  - [x] `dig +short home.bry-guy.net CNAME` -> `us-west-appliance-500.tail9e13b.ts.net.`
  - [x] `curl https://home.bry-guy.net/api/healthcheck` -> HTTP 200 `up`
- [x] Confirm Homepage k3s deployment is healthy.
  - [x] `mise run selfhost:homepage:status`
  - [x] Fixed restart loop by mounting empty `proxmox.yaml`; Homepage was trying to copy the skeleton file into read-only `/app/config`.
- [x] Confirm Grafana and observability stack are healthy.
  - [x] `mise run selfhost:observability:status`
  - [x] Observability pods are running and core ArgoCD apps are healthy (`kube-prometheus-stack`, `loki`, `promtail`, `opentelemetry-collector`, `postgres-exporter`).
- [x] Confirm Gatus/status page is healthy.
  - [x] `curl https://status.bry-guy.net/` -> HTTP 200
- [x] Confirm Caddy appliance route config still serves both:
  - [x] `https://admin.bry-guy.net/login` -> HTTP 200
  - [x] `https://home.bry-guy.net/api/healthcheck` -> HTTP 200 `up`

### Ownership boundaries

- [x] Keep Homepage deployment, Caddy, Gatus, Prometheus rules, Grafana dashboards, and platform scrape config in `~/dev/infra`.
- [x] Keep Castaway app instrumentation and Castaway deploy ServiceMonitors/manifests in `~/dev/srvivor` unless a scrape target is clearly platform-owned.
- [x] Do not put API tokens/secrets in Homepage ConfigMaps.
- [x] Do not expose Prometheus/Loki/Argo CD/Alertmanager directly through `home.bry-guy.net`.

## Phase 1 — Homepage catalog completeness

Goal: `home.bry-guy.net` accurately lists current homelab services.

Status: mostly done 2026-05-20. Homepage catalog was expanded and applied. Dashboard deep-links remain placeholders until dashboards exist.

### Inventory review

- [x] List all current homelab services and decide their Homepage group:
  - [x] Home
  - [x] Admin / Grafana
  - [x] Status / Gatus
  - [x] Castaway Web API
  - [x] Castaway Discord Bot
  - [x] Argo CD
  - [x] Prometheus via Grafana Explore
  - [x] Loki via Grafana Explore
  - [x] Alertmanager via Grafana alerting
  - [x] OpenTelemetry Collector
  - [x] PostgreSQL Exporter
  - [x] Shared PostgreSQL
  - [x] Caddy Edge
  - [x] SFFPC Proxmox
  - [x] OptiPlex Proxmox
  - [x] Pi witness / qnetd
  - [x] JetKVM witness
  - [x] Router
  - [x] k3s/platform VMs
- [x] For each entry, record:
  - [x] display name
  - [x] group
  - [x] URL or explicit “no direct URL” note
  - [x] health source in description when applicable
  - [ ] Grafana dashboard link, if available — pending dashboard creation
  - [x] owning repo/source of truth implied by infra Homepage config or app namespace

### Homepage config

- [x] Update `infra:selfhost/platform/homepage/configmap.yaml` with missing services.
- [x] Add clear descriptions for services without direct UIs.
- [ ] Add Grafana dashboard links once dashboards exist.
- [x] Apply Homepage config.
  - [x] `cd ~/dev/infra && mise run selfhost:homepage:k8s:apply`
- [x] Smoke test Homepage.
  - [x] `curl https://home.bry-guy.net/api/healthcheck` -> HTTP 200
  - [ ] open `https://home.bry-guy.net/` manually in browser

## Phase 2 — Edge metrics for home/admin/status

Goal: Grafana can show usage and health of `home`, `admin`, and `status` entrypoints.

Status: mostly implemented 2026-05-21. Live Caddy was updated through a privileged k8s host command because Tailscale SSH still required an interactive check. Home/admin edge metrics are scraped; status edge metrics remain future work because status is served by witness Caddy.

### Caddy metrics

- [x] Decide scrape shape:
  - [ ] Preferred first pass: scrape Caddy admin metrics on the appliance via tailnet/static target.
  - [x] Selected: Caddy metrics handler bound only to the appliance Tailscale IP on port 9180.
- [x] Confirm Caddy metrics endpoint works on appliance.
  - [ ] `ssh fedora@us-west-appliance-500.tail9e13b.ts.net 'curl -s localhost:2019/metrics | head'` — still blocked by Tailscale SSH check.
  - [x] `curl http://100.123.197.8:9180/metrics` -> Caddy metrics with per-host labels.
- [x] Add Prometheus scrape config for Caddy metrics in infra observability config.
  - [x] Added `selfhost/platform/observability/external-servicemonitors.yaml` with Service/Endpoints/ServiceMonitor for `100.123.197.8:9180`.
  - [x] Updated `scripts/selfhost-observability-apply.sh` to apply external ServiceMonitors.
- [x] Apply observability config.
  - [ ] `cd ~/dev/infra && mise run selfhost:observability:apply` — full task blocked by missing local `SELFHOST_GRAFANA_ADMIN_PASSWORD`; applied Caddy external ServiceMonitor directly with kubectl.
- [x] Verify Prometheus target is up.
- [x] Verify metrics exist for edge requests/reloads.

### Caddy logs, optional but useful

- [x] Decide whether to ship Caddy JSON access logs now or after metrics dashboard exists.
  - [x] Decision: defer logs until Caddy metrics dashboard is working.
- [ ] If now:
  - [ ] Configure Caddy per-site JSON logs for home/admin/status.
  - [ ] Configure Promtail/journald/file scrape from appliance.
  - [ ] Confirm logs land in Loki with host labels.
  - [ ] Add LogQL examples for edge troubleshooting.

### Edge dashboard

- [x] Create Grafana dashboard ConfigMap: Edge / entrypoints.
- [x] Add panels:
  - [x] requests/min by host
  - [x] p95 latency by host
  - [x] status class by host
  - [x] 4xx/5xx by host
  - [x] Caddy reload success
  - [ ] recent edge errors from Loki, if logs are enabled
- [x] Add Homepage entry/dashboard link for Edge / entrypoints.
- [x] Apply dashboards.
  - [ ] `cd ~/dev/infra && mise run selfhost:observability:dashboards:sync` — not needed; applied dashboard ConfigMap directly and verified Grafana loaded it.

## Phase 3 — Gatus/status integration

Goal: external/tailnet health checks are visible in both Gatus and Grafana.

### Gatus checks

- [x] Add Gatus endpoint for Homepage:
  - [x] name: `home-page`
  - [x] URL: `https://home.bry-guy.net/api/healthcheck`
  - [x] condition: status 200
- [x] Add/confirm Gatus endpoint for Grafana:
  - [x] name: `admin-grafana`
  - [x] URL: `https://admin.bry-guy.net/login`
  - [x] condition: status 200
- [ ] Add Castaway web health endpoint once reachable from witness.
- [ ] Add Castaway bot health endpoint after bot `/healthz` exists.
- [ ] Deploy Pi witness config — pending witness deployment/auth path.
- [ ] Confirm `https://status.bry-guy.net/` shows the new checks.

### Gatus metrics scrape

- [x] Add Prometheus scrape target for Gatus `/metrics` on the Pi witness.
  - [x] Confirmed `https://status.bry-guy.net/metrics` exposes Gatus metrics from tailnet clients.
  - [x] Added CoreDNS `coredns-custom` tailnet host mapping for `status.bry-guy.net` so in-cluster scrapes resolve to the witness tailnet IP.
- [x] Prefer Kubernetes `Endpoints` + `ServiceMonitor` in an infra-owned namespace, or a static scrape config.
  - Implemented as Prometheus Operator `ScrapeConfig` `gatus-witness`.
- [x] Apply observability config.
- [x] Verify Gatus metrics in Prometheus.

### Witness/status dashboard

- [ ] Create Grafana dashboard ConfigMap: Witness / status.
- [ ] Add panels:
  - [ ] endpoint up/down table
  - [ ] failing endpoints
  - [ ] success rate by endpoint/group
  - [ ] p95 check latency
  - [ ] qnetd TCP check status
- [ ] Add Homepage dashboard link for status/witness.

## Phase 4 — Castaway web metrics and structured logs

Goal: web API usage, latency, errors, and DB health are visible in Grafana.

Status: implemented in repo 2026-05-20, not deployed. Unit/package tests for `castaway-web` pass. Full CI/deploy remains.

### App instrumentation

- [x] Add Prometheus dependencies to `castaway-web`.
- [x] Add `/metrics` endpoint.
- [x] Add HTTP middleware metrics:
  - [x] `castaway_web_http_requests_total{method,route,status_class}`
  - [x] `castaway_web_http_request_duration_seconds_bucket{method,route}`
  - [x] `castaway_web_http_in_flight_requests`
- [x] Add DB pool metrics from `pgxpool.Stat()`.
- [x] Add DB dependency gauge:
  - [x] `castaway_web_db_up`
- [x] Ensure route labels are route templates, not raw paths with IDs.
- [ ] Add tests for metrics middleware where practical.

### Logs

- [x] Replace Gin default logger with structured request logger.
- [x] Suppress successful `/healthz` logs.
- [x] Include fields:
  - [x] service
  - [x] method
  - [x] route
  - [x] status
  - [x] status_class
  - [x] duration_ms
  - [x] request_id
  - [x] error_class when relevant
- [x] Confirm no auth headers, tokens, request bodies, or secrets are logged.

### Deployment

- [x] Add ServiceMonitor for `castaway-web`.
- [x] Add OTEL resource env vars:
  - [x] `OTEL_SERVICE_NAME=castaway-web`
  - [x] `OTEL_RESOURCE_ATTRIBUTES=deployment.environment=home-k3s,service.namespace=castaway`
- [ ] Run Castaway CI.
  - [ ] `cd ~/dev/srvivor && mise run ci`
  - [x] `cd ~/dev/srvivor && mise exec -- bash -lc 'cd apps/castaway-web && go test ./...'`
- [ ] Deploy Castaway.
- [ ] Verify Prometheus sees `castaway_web_http_requests_total`.
- [ ] Verify Loki shows structured web logs without healthz spam.

## Phase 5 — Castaway Discord bot health and metrics

Goal: Discord command usage and dependency failures are visible in Grafana.

Status: mostly implemented in repo 2026-05-21, not deployed. Bot package tests pass. API-client metrics and reconnect counter remain follow-ups.

### Bot HTTP server

- [x] Add internal HTTP server on `:8080`.
- [x] Add `GET /healthz`.
  - [x] 200 when process is running and Discord session is open/recently healthy.
  - [x] non-200 only for durable unhealthy state.
- [x] Add `GET /metrics`.

### Command metrics

- [x] Add command counter:
  - [x] `castaway_bot_commands_total{group,command,result}`
- [x] Add command duration histogram:
  - [x] `castaway_bot_command_duration_seconds_bucket{group,command}`
- [x] Add API client request counter:
  - [x] `castaway_bot_api_requests_total{method,route,status_class,result}`
- [x] Add API client request latency histogram:
  - [x] `castaway_bot_api_request_duration_seconds_bucket{method,route}`
- [x] Add Discord gateway gauge:
  - [x] `castaway_bot_discord_gateway_connected`
- [ ] Add reconnect counter if discordgo supports a reliable hook.

### Bot logs

- [x] Log one completion summary per command.
- [x] Include:
  - [ ] service — command log records logger/process context but not explicit service field yet
  - [x] group
  - [x] command
  - [x] result
  - [x] duration_ms
  - [ ] dependency_error_class when relevant — result classification is present; explicit class field remains follow-up
- [x] Do not log Discord user/guild IDs by default.

### Deployment

- [x] Add bot Service.
- [x] Add readiness/liveness probes.
- [x] Add ServiceMonitor.
- [x] Add OTEL resource env vars.
- [ ] Run Castaway CI.
  - [ ] `cd ~/dev/srvivor && mise run ci`
  - [x] `cd ~/dev/srvivor && mise exec -- bash -lc 'cd apps/castaway-discord-bot && go test ./...'`
- [ ] Deploy Castaway.
- [ ] Verify Prometheus sees `castaway_bot_commands_total`.
- [ ] Verify Loki shows command completion logs.

## Phase 6 — Castaway dashboard and alerts

Goal: one Castaway dashboard answers health, usage, latency, and errors.

Status: dashboard implemented and loaded 2026-05-21. Some panels will remain empty until Castaway app instrumentation is deployed.

### Dashboard

- [x] Create Castaway overview dashboard ConfigMap.
- [x] Add Health row:
  - [x] web pod ready
  - [x] bot pod ready
  - [x] web DB up
  - [x] bot gateway connected
  - [x] restarts last 24h
- [x] Add Usage row:
  - [x] web requests/min by route top 5
  - [x] Discord commands/min by command top 10
  - [x] command success vs failure
- [x] Add Latency row:
  - [x] web p95 HTTP latency
  - [x] bot p95 command latency
  - [x] bot -> web API p95 latency
- [x] Add Errors row:
  - [x] web 5xx/min
  - [x] bot dependency/internal errors/min
  - [ ] recent error logs
- [x] Add dashboard link to Homepage Castaway entries.

### Alerts

- [x] Add `CastawayWebDown` (`CastawayWebUnavailable`).
- [x] Add `CastawayBotDown` (`CastawayDiscordBotUnavailable`).
- [x] Add `CastawayWebHighErrorRate` (`CastawayWebHigh5xxRate`).
- [ ] Add `CastawayBotCommandFailures`.
- [ ] Add `CastawayWebHighLatency`.
- [x] Reuse existing Postgres down/scrape alerts.
- [x] Verify alert thresholds require enough traffic to avoid low-volume noise.

## Phase 7 — Homelab overview dashboard

Goal: one Grafana dashboard answers “what is broken?” within one screen.

- [x] Create Homelab overview dashboard ConfigMap.
- [x] Add Entry points row:
  - [x] Homepage up
  - [x] Grafana/admin up
  - [x] Status/Gatus up
  - [x] Caddy 5xx by host
- [x] Add Applications row:
  - [x] Castaway web up
  - [x] Castaway bot up
  - [x] command success rate/failure rate
  - [x] web 5xx rate
- [x] Add Platform row:
  - [x] k3s nodes ready
  - [ ] Argo CD unhealthy/out-of-sync apps
  - [x] pod restarts/crashloops
  - [x] PVCs near full
- [x] Add Stateful row:
  - [x] PostgreSQL up
  - [ ] PostgreSQL connections
  - [ ] postgres-exporter scrape errors
- [x] Add Witness/substrate row:
  - [x] Gatus failing endpoints
  - [x] qnetd reachable
  - [x] Proxmox API checks
- [x] Make this the primary dashboard linked from Homepage.

## Phase 8 — Platform capacity polish

Goal: the admin can understand capacity and platform component health without digging through raw Kubernetes dashboards.

- [ ] Add/verify panels for k3s node CPU/memory.
- [ ] Add namespace CPU/memory panels.
- [ ] Add pod restart panels by namespace.
- [ ] Add pending pod panel.
- [ ] Add PVC fullness panel.
- [ ] Add Argo CD app health/sync panels.
- [ ] Add OTel Collector receiver/exporter error panels.
- [ ] Add Loki ingestion/error panels if chart metrics are available.
- [ ] Add PostgreSQL usage panels.
- [ ] Link relevant Homepage entries to the dashboard/panel.

## Phase 9 — Optional Pi host metrics

Goal: the Pi witness is visible as a host, not just as Gatus/qnetd endpoints.

- [ ] Decide whether to add node-exporter to the Pi witness OS config.
- [ ] If yes:
  - [ ] Enable node-exporter declaratively.
  - [ ] Open only needed tailnet/LAN scrape port.
  - [ ] Add Prometheus scrape target.
  - [ ] Add dashboard panels for CPU/memory/disk.
  - [ ] Add low-noise disk-full alert.
- [ ] If no, document Gatus-only witness monitoring as intentional for now.

## Phase 10 — Optional Proxmox and Blocky deepening

### Proxmox

- [ ] Decide if Proxmox exporter is worth adding now.
- [ ] Design API token storage before implementing.
- [ ] If implemented:
  - [ ] Add exporter deployment or host service.
  - [ ] Add scrape config.
  - [ ] Add panels for node online, VM status, storage fullness, backup freshness.
  - [ ] Add Homepage links/widgets only if tokens are handled safely.

### Blocky

- [ ] Do nothing while Blocky is disabled.
- [ ] When Blocky is enabled:
  - [ ] Scrape Blocky `/metrics`.
  - [ ] Add DNS dashboard panels:
    - [ ] queries/sec
    - [ ] blocked queries/sec
    - [ ] upstream error rate
    - [ ] p95 DNS response time
  - [ ] Add Gatus DNS check.
  - [ ] Add alert only if DNS is intended to be active.

## Final acceptance checklist

### Homepage

- [ ] `https://home.bry-guy.net/` loads over HTTPS only.
- [ ] Homepage lists all current homelab services.
- [ ] Services without direct UIs have useful descriptions.
- [ ] Homepage links to Grafana dashboards where available.
- [ ] No secrets are present in Homepage ConfigMaps.

### Grafana

- [ ] Homelab overview dashboard exists.
- [ ] Edge / entrypoints dashboard exists.
- [ ] Castaway overview dashboard exists.
- [ ] Witness / status dashboard exists.
- [ ] Platform capacity dashboard exists or existing dashboards cover it clearly.
- [ ] Homepage usage is visible via `home.bry-guy.net` edge metrics/logs.

### Metrics

- [ ] Prometheus has Caddy edge metrics.
- [ ] Prometheus has Gatus metrics.
- [ ] Prometheus has Castaway web metrics.
- [ ] Prometheus has Castaway bot metrics.
- [ ] Prometheus has platform PostgreSQL metrics.

### Logs

- [ ] Loki has Caddy edge logs if enabled.
- [ ] Loki has structured Castaway web logs without healthz spam.
- [ ] Loki has Castaway bot command completion logs.

### Alerts

- [x] Home/admin/status critical availability alerts exist.
- [x] Castaway web/bot alerts exist.
- [x] Gatus important endpoint alerts exist.
- [x] qnetd alert exists.
- [x] Alerts avoid user-error and low-volume noise.

### Admin questions

- [ ] Can I tell whether Homepage is up and used?
- [ ] Can I tell whether Castaway is healthy and used?
- [ ] Can I tell whether failures are edge, app, DB, Discord, DNS/network, or platform related?
- [ ] Can I tell whether the Pi witness/qnetd is healthy?
- [ ] Can I tell whether k3s nodes, pods, and PVCs are healthy?
