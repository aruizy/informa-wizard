---
name: mini-prd-creator-monday
description: >
  Create a mini-PRD in Spanish via a clarification interview, then persist it as a
  main item with subtasks on a Monday.com board. Requires the Monday MCP server to
  be configured (component: monday).
  Trigger: When the user wants to land a request into a compact mini-PRD, needs a
  clarification interview in Spanish, or wants to create/update a main task with
  subtasks in Monday from a brief.
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "1.0"
---

## When to Use

Use this skill when:
- The user wants to turn a request into a compact mini-PRD
- The user needs a clarification interview in Spanish to define scope
- The user wants to create a main task with subtasks in Monday.com
- The user says "mini-prd", "aterrizar tarea", "crear tarea en Monday"

---

## Prerequisites

- The Monday MCP server must be configured (installed via `--component monday`)
- A default board ID must be available (provided via `--monday-board` during install or asked at runtime)
- Monday MCP tools are available: `mcp__claude_ai_monday_com__create_item`, `mcp__claude_ai_monday_com__get_board_info`, etc.

---

## Workflow

### Phase 1: Clarification Interview (Spanish)

Conduct a short interview in Spanish (3-5 questions max) to extract:

1. **Objetivo**: ¿Qué se quiere lograr?
2. **Alcance**: ¿Qué incluye y qué NO incluye?
3. **Criterios de aceptación**: ¿Cómo sabemos que está terminado?
4. **Dependencias**: ¿Depende de algo externo o de otra tarea?
5. **Prioridad**: ¿Es urgente, importante, o puede esperar?

Keep questions focused. Skip questions whose answers are obvious from context.

### Phase 2: Mini-PRD Generation

Generate a compact mini-PRD document in Spanish with this structure:

```
## Mini-PRD: {título conciso}

**Objetivo:** {una línea}

**Alcance:**
- Incluye: {bullet points}
- No incluye: {bullet points}

**Criterios de aceptación:**
- [ ] {criterio 1}
- [ ] {criterio 2}
- ...

**Dependencias:** {lista o "ninguna"}

**Prioridad:** {alta/media/baja}

**Subtareas:**
1. {subtarea 1}
2. {subtarea 2}
3. ...
```

### Phase 3: Persist to Monday.com

1. **Get board info** to understand columns and groups:
   ```
   mcp__claude_ai_monday_com__get_board_info(boardId: "{board_id}")
   ```

2. **Create the main item** with the mini-PRD title:
   ```
   mcp__claude_ai_monday_com__create_item(
     boardId: "{board_id}",
     itemName: "{título del mini-PRD}",
     columnValues: { "status": "Working on it" }
   )
   ```

3. **Create subtask items** as subitems of the main item:
   ```
   mcp__claude_ai_monday_com__create_item(
     boardId: "{board_id}",
     itemName: "{subtarea}",
     parentItemId: "{main_item_id}"
   )
   ```

4. **Add the full mini-PRD as an update** (comment) on the main item:
   ```
   mcp__claude_ai_monday_com__create_update(
     itemId: "{main_item_id}",
     body: "{mini-PRD completo en markdown}"
   )
   ```

---

## Board ID Resolution

The board ID is resolved in this order:
1. User provides it explicitly in the conversation
2. Read from project config (`.informa-wizard/monday.json` in the workspace root if exists)
3. Read from global wizard config (`~/.informa-wizard/monday.json` if exists)
4. Ask the user: "¿En qué tablero de Monday quieres crear la tarea? Dame el board ID."

---

## Output

After creating the items, show the user:
- Main item name and ID
- Number of subtasks created
- Link to the board (if available)
- The full mini-PRD text for reference
