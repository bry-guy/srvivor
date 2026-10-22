# Running the infra "migration" DNS apply would replace the Castaway tunnel and take castaway.bry-guy.net down

Found: 2026-10-12, while adding an unrelated DNS record in `~/dev/infra`. Status: open (nothing applied).

## What happens
`mise run selfhost:platform:migration:dns:plan` in `~/dev/infra` shows:

- `cloudflare_zero_trust_tunnel_cloudflared.castaway[0]` **must be replaced** (`account_id = "…" -> null # forces replacement`)
- `cloudflare_zero_trust_tunnel_cloudflared_config.castaway[0]` updated (new tunnel id)
- `cloudflare_record.castaway` updated (CNAME → the new tunnel's `cfargotunnel.com` name)
- output `castaway_tunnel_token` changes

If someone runs the matching `…:migration:dns:apply`, Cloudflare gets a new tunnel with a new token. The
cloudflared connector in the cluster still holds the old token, so castaway.bry-guy.net stops serving until the
connector secret is updated and redeployed.

## Cause
`cloudflare/dns/castaway_tunnel.tf` sets `account_id = var.cloudflare_account_id`. The canonical tasks
`selfhost:cloudflare-dns:plan|apply` export `TF_VAR_cloudflare_account_id="$CLOUDFLARE_ACCOUNT_ID"`; the older
`selfhost:platform:migration:dns:plan|apply` tasks do not, so the variable is null there.
`mise run selfhost:cloudflare-dns:plan` shows no Castaway changes, so the live tunnel and state are fine.

## Fix (in ~/dev/infra)
Either export `TF_VAR_cloudflare_account_id` in the migration DNS tasks, or remove those tasks in favor of
`selfhost:cloudflare-dns:*`. Optionally make the variable required (no default/nullable = false) so a missing
value fails the plan instead of planning a replacement, and add `lifecycle { prevent_destroy = true }` to the
tunnel resource.

## Check after the fix
`mise run selfhost:platform:migration:dns:plan` (if kept) shows no `castaway` changes.
