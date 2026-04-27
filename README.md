<div align="center">

<h1>Informa Wizard</h1>

<p><strong>Configure your AI coding agents in one command. SDD, skills, agents, and MCP servers — ready to go.</strong></p>

<p>
<a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a>
<img src="https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white" alt="Go 1.24+">
<img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey" alt="Platform">
</p>

</div>

---

## What It Does

This is an **ecosystem configurator** -- it takes whatever AI coding agent(s) you use and supercharges them with the Informa Wizard stack: persistent memory, Spec-Driven Development workflow, curated coding skills, MCP servers (including Monday.com integration), security-first permissions, and per-phase model assignment so each SDD step can run on a different model.

**Before**: "I installed Claude Code / OpenCode / VS Code, but it's just a chatbot that writes code."

**After**: Your agent now has memory, skills, workflow, MCP tools, and Monday.com task management integrated into the development cycle.

### Supported Agents

| Agent | Delegation Model | Key Feature |
|-------|:---:|---|
| **Claude Code** | Full (Task tool) | Sub-agents, custom agents in `~/.claude/agents/` |
| **OpenCode** | Full (multi-mode overlay) | Per-phase model routing, JSON agent definitions |
| **VS Code Copilot** | Full (runSubagent) | Agent files in `prompts/`, parallel execution |

---

## Pre-requisites

Install Go 1.24+ if not already present:

```bash
# Windows (chocolatey)
choco install golang

# macOS (homebrew)
brew install go

# Linux
sudo apt install golang  # or your package manager
```

Verify: `go version`

> **Important:** Use the stable version of VS Code, not VS Code Insiders. The Insiders build may have compatibility issues with the wizard's agent and MCP configuration.

---

## Install

```bash
git clone https://gitlab.informa.tools/ai/wizard/informa-wizard.git
cd informa-wizard
go install ./cmd/informa-wizard
```

---

### After install: execute wizard

```bash
informa-wizard
```

---

## Installation Walkthrough

<details>
<summary><strong>Click to view the installation flow screenshots</strong></summary>

**1. Welcome menu**

![Welcome](docs/images/01-welcome.png)

**2. System detection** — checks tools, dependencies, and which agent configs are present

![System Detection](docs/images/02-detection.png)

**3. Select AI agents** — pick the agents to configure

![Select AI Agents](docs/images/03-agents.png)

**4. Select ecosystem preset** — Full, Ecosystem or Custom

![Select Preset](docs/images/04-preset.png)

**5. Strict TDD mode**

![Strict TDD](docs/images/05-strict-tdd.png)

**6. Configure Claude models** — assign models per SDD phase

![Claude Models](docs/images/06-claude-models.png)

**7. Select skills** — pick the curated coding skills to install

![Select Skills](docs/images/07-skills.png)

**8. Select dev-skills** — external skills from the shared dev-skills repo

![Select Dev Skills](docs/images/08-dev-skills.png)

**9. Select dev-agents** — external orchestrator agents from dev-orchestrators

![Select Dev Agents](docs/images/09-dev-agents.png)

**10. Review and confirm** — final summary before installing

![Review and Confirm](docs/images/10-review.png)

</details>

---

### After wizard install: project-level setup

Once your agents are configured, open your AI agent in a project and run these two commands to register the project context:

| Command | What it does | When to re-run |
|---------|-------------|----------------|
| `/sdd-init` | Detects stack, testing capabilities, activates Strict TDD Mode if available | When your project adds/removes test frameworks, or first time in a new project |
| `/skill-registry` | Scans installed skills and project conventions, builds the registry | After installing/removing skills, or first time in a new project |

These are **not required** for basic usage. The SDD orchestrator runs `/sdd-init` automatically if it detects no context. But if something changed in your project (new test runner, new dependencies), re-running them manually ensures the agents have up-to-date context.

---

## Monday.com Integration

Informa Wizard includes built-in Monday.com integration. During install, provide your credentials:

```bash
informa-wizard install --component monday --monday-token "your-api-token" --monday-board "board-id"
```

This configures the Monday MCP server for all your agents. The SDD cycle then automatically:
- **sdd-tasks**: Searches for existing Monday items or creates new ones with subtasks
- **sdd-apply**: Updates subtask status to Done as tasks are completed
- **sdd-verify**: Sets the item to Done or Stuck based on verification results

---

## Backups

Every install, sync, and upgrade automatically snapshots your agent config directories. Backups are **compressed** (tar.gz), **deduplicated** (identical configs are not re-backed up), and **auto-pruned** (keeps the 5 most recent). Pin important backups via the TUI (`p` key) to protect them from pruning.

See [Backup & Rollback Guide](docs/rollback.md) for details.

---

## Documentation

| Topic | Description |
|-------|-------------|
| [Intended Usage](docs/intended-usage.md) | How informa-wizard is meant to be used — the mental model |
| [Agents](docs/agents.md) | Supported agents, feature matrix, config paths, and per-agent notes |
| [Components, Skills & Presets](docs/components.md) | All components, GGA behavior, skill catalog, and preset definitions |
| [Usage](docs/usage.md) | Interactive TUI, CLI flags, and dependency management |
| [Backup & Rollback](docs/rollback.md) | Backup retention, compression, dedup, pinning, and restore |
| [Platforms](docs/platforms.md) | Supported platforms, Windows notes, security verification, config paths |
| [Architecture & Development](docs/architecture.md) | Codebase layout, testing, and development |

---

<div align="center">
<a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a>
</div>
