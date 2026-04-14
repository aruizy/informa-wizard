---
name: sdd-new
description: >
  Start a new SDD change — runs exploration then creates a proposal.
  Trigger: When user says "sdd-new", "new change", "start a change", or wants to begin a new SDD cycle.
license: MIT
metadata:
  author: gentleman-programming
  version: "1.0"
---

## Orchestrator Meta-Command

This is a **meta-command** handled by the SDD orchestrator (you). Do NOT delegate this to a sub-agent.

When the user invokes `/sdd-new <change-name>`, you are the orchestrator and you must:

1. **Check SDD init**: Search for `sdd-init/{project}` context. If not found, run `/sdd-init` first.
2. **Ask artifact store** (if not cached for this session): engram, openspec, hybrid, or use the default from the Artifact Store Policy.
3. **Run the definition pipeline** in plan-build mode (or the user's chosen mode):
   - Delegate `sdd-explore` (investigate the change)
   - Delegate `sdd-propose` (create a proposal)
   - Delegate `sdd-spec` (write specifications)
   - Delegate `sdd-design` (technical design)
   - Delegate `sdd-tasks` (task breakdown)
4. **Stop at PRE-IMPLEMENTATION GATE** — present the plan summary and ask the user to confirm before proceeding to `sdd-apply`.

The `<change-name>` argument becomes the identifier for all artifacts (e.g., `sdd/<change-name>/proposal`).

If no change name is provided, ask the user: "What change do you want to start? Give it a short name."

Follow all orchestrator rules from the SDD Orchestrator section in your system prompt (execution modes, halt conditions, flow control, sub-agent context protocol).
