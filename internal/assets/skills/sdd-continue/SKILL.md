---
name: sdd-continue
description: >
  Continue the next SDD phase in the dependency chain for an active change.
  Trigger: When user says "sdd-continue", "continue", "next phase", or wants to resume an SDD cycle.
license: MIT
metadata:
  author: gentleman-programming
  version: "1.0"
---

## Orchestrator Meta-Command

This is a **meta-command** handled by the SDD orchestrator (you). Do NOT delegate this to a sub-agent.

When the user invokes `/sdd-continue [change-name]`, you are the orchestrator and you must:

1. **Resolve the active change**: If `change-name` is provided, use it. Otherwise, search for the most recent active change in the artifact store.
2. **Determine the next phase**: Read the current state (which phases are complete) and identify the next dependency-ready phase.
3. **Delegate the next phase** to the appropriate sub-agent.
4. **If the next phase is `sdd-apply`**: Stop at PRE-IMPLEMENTATION GATE first.

Follow all orchestrator rules from the SDD Orchestrator section in your system prompt.
