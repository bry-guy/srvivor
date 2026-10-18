#!/usr/bin/env bash
# Pin castaway-web digests in the home-k3s overlays on main, then wait until the
# environment serves $SHA. Argo CD syncs deploy/environments/* from main.
# Usage: SHA=<commit> WEB_DIGEST=sha256:<hex> [BOT_DIGEST=...] [PROBST_DIGEST=...] scripts/deploy.sh preview|production
# preview pins WEB_DIGEST only. production pins every digest that is set and waits
# only when WEB_DIGEST is set (a bot-only change does not move castaway-web's build).
set -euo pipefail

env_name="${1:?usage: deploy.sh preview|production}"
case "$env_name" in
  preview) overlay=deploy/environments/home-k3s-preview; url=https://castaway.preview.bry-guy.net/healthz ;;
  production) overlay=deploy/environments/home-k3s; url=https://castaway-origin.bry-guy.net/healthz ;; # tailnet origin: the public edge's Bot Fight Mode blocks CI
  *) echo "unknown environment: $env_name" >&2; exit 2 ;;
esac
digest_re='^sha256:[0-9a-f]{64}$'
image_prefix="ghcr.io/${GITHUB_REPOSITORY_OWNER:-bry-guy}"
[[ "${SHA:-}" =~ ^[0-9a-f]{40}$ ]] || { echo "SHA must be a 40-character commit SHA" >&2; exit 2; }
if [ "$env_name" = preview ] || [ -n "${WEB_DIGEST:-}" ]; then
  [[ "${WEB_DIGEST:-}" =~ $digest_re ]] || { echo "WEB_DIGEST must be sha256:<64 hex>" >&2; exit 2; }
fi
for d in "${BOT_DIGEST:-}" "${PROBST_DIGEST:-}"; do
  [ -z "$d" ] || [[ "$d" =~ $digest_re ]] || { echo "digests must be sha256:<64 hex>" >&2; exit 2; }
done

git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"

pin() {
  if [ "$env_name" = preview ]; then
    # The preview overlay has exactly one digest line (castaway-web).
    sed -E "s|(digest: )sha256:[0-9a-f]{64}|\1$WEB_DIGEST|" "$overlay/kustomization.yaml" > "$overlay/kustomization.yaml.tmp"
    mv "$overlay/kustomization.yaml.tmp" "$overlay/kustomization.yaml"
    return
  fi
  [ -z "${WEB_DIGEST:-}" ] || python3 script/update-home-k3s-digests.py "$image_prefix/castaway-web" "$WEB_DIGEST"
  [ -z "${BOT_DIGEST:-}" ] || python3 script/update-home-k3s-digests.py "$image_prefix/castaway-discord-bot" "$BOT_DIGEST"
  [ -z "${PROBST_DIGEST:-}" ] || python3 script/update-home-k3s-digests.py "$image_prefix/probst" "$PROBST_DIGEST"
}

for attempt in 1 2 3 4 5; do
  git fetch --quiet origin main
  git reset --quiet --hard origin/main
  pin
  if git diff --quiet; then
    echo "$env_name already pins these digests"
    break
  fi
  kubectl kustomize "$overlay" > /dev/null
  git commit --quiet -m "deploy($env_name): ${SHA:0:7} [skip ci]" -m "Source: $GITHUB_SERVER_URL/$GITHUB_REPOSITORY/commit/$SHA" -- "$overlay/kustomization.yaml"
  if git push --quiet origin HEAD:main; then
    echo "Pushed $env_name pin for ${SHA:0:7}"
    # Best-effort: Argo CD refreshes now instead of at its next 30s poll.
    curl -fsS -m 10 -X POST https://argocd-hook.bry-guy.net/api/webhook -H 'Content-Type: application/json' -H 'X-GitHub-Event: push' -d "{\"ref\":\"refs/heads/main\",\"after\":\"$(git rev-parse HEAD)\",\"repository\":{\"html_url\":\"https://github.com/$GITHUB_REPOSITORY\",\"default_branch\":\"main\"},\"commits\":[]}" >/dev/null || echo "argo webhook unavailable; Argo will poll"
    break
  fi
  [ "$attempt" -lt 5 ] || { echo "could not push the $env_name pin" >&2; exit 1; }
  sleep $((attempt * 3))
done

[ -n "${WEB_DIGEST:-}" ] || { echo "castaway-web unchanged; not waiting for $env_name"; exit 0; }

# The webhook above usually beats Argo's 30s Git poll; allow 10 minutes for pickup plus rollout.
for _ in $(seq 1 200); do
  build="$(curl --fail --silent --connect-timeout 5 --max-time 10 "$url" | jq -r '.buildId // empty' || true)"
  if [ "$build" = "$SHA" ]; then
    echo "$env_name is serving ${SHA:0:7}"
    exit 0
  fi
  sleep 3
done
echo "$env_name did not serve ${SHA:0:7} within 10 minutes (last: ${build:-unreachable}); check Argo CD" >&2
exit 1
