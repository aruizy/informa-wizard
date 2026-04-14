### Artifact Store Policy

- `engram` — default; persistent memory across sessions, cross-session recovery
- `openspec` — file-based artifacts, committable, shareable with team, full git history
- `hybrid` — both backends; cross-session recovery + local files; more tokens per op
- `none` — return results inline only; recommend enabling engram or openspec

If the user doesn't specify an artifact store, default to `engram`.
