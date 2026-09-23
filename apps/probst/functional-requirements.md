# Functional requirements

- Inspect delegated service identity, instances, players, drafts, and public scores.
- Bind/unbind Discord channels and link/unlink player identities with explicit IDs.
- Bootstrap the first instance admin only through the API's configured bootstrap identity.
- Require `--yes` for channel replacement, unbinding, and unlinking.
- Support readable output and `--json`; propagate HTTP failures as nonzero exits.
- Defer season operations, scheduling, and gameplay CRUD.
