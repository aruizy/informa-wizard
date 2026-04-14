---
name: sdd-ff
description: >
  Fast-forward all SDD planning phases — proposal through tasks — without pausing between them.
  Trigger: When user says "sdd-ff", "fast forward", "ff", or wants to run all planning phases at once.
license: MIT
metadata:
  author: gentleman-programming
  version: "1.0"
---

## Orchestrator Meta-Command

This is a **meta-command** handled by the SDD orchestrator (you). Do NOT delegate this to a sub-agent.

When the user invokes `/sdd-ff <change-name>`, you are the orchestrator and you must:

1. **Check SDD init**: Search for `sdd-init/{project}` context. If not found, run `/sdd-init` first.
2. **Ask artifact store** (if not cached for this session): engram, openspec, hybrid, or use the default from the Artifact Store Policy.
3. **Run ALL definition phases continuously** without pausing (unless a HALT CONDITION is detected):
   - Delegate `sdd-propose` (create a proposal)
   - Delegate `sdd-spec` (write specifications)
   - Delegate `sdd-design` (technical design)
   - Delegate `sdd-tasks` (task breakdown)
4. **ALWAYS stop at PRE-IMPLEMENTATION GATE** — even in fast-forward mode, present the plan summary and wait for user confirmation before `sdd-apply`.

The `<change-name>` argument becomes the identifier for all artifacts.

If no change name is provided, ask the user: "What change do you want to fast-forward? Give it a short name."

Follow all orchestrator rules from the SDD Orchestrator section in your system prompt.
