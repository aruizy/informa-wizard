### Artifact Store Policy

- `openspec` — default; file-based artifacts, committable, shareable with team, full git history
- `engram` — persistent memory across sessions; use when user explicitly requests
- `hybrid` — both backends; cross-session recovery + local files; more tokens per op
- `none` — return results inline only; recommend enabling engram or openspec

If the user doesn't specify an artifact store, default to `openspec`.
