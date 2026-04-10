---
name: jeeves
description: Research a topic using the Jeeves CLI
trigger: when user asks to research, learn about, or look up a topic
---

Use the `jeeves` CLI to research topics. Always use `--json` for structured output.

## Quick lookup (fast, no LLM)
```bash
jeeves research "query" -r glance --json
```

## Standard research (local LLM summary)
```bash
jeeves research "query" -r brief --json
```

## Deep research (Claude synthesis, persists to knowledge base)
```bash
jeeves research "query" -r detailed --topic <slug> --json
```

## Check what's already known
```bash
jeeves wiki search "query" --json
```

## See topic knowledge map
```bash
jeeves topic outline <slug> --json
```

## Find gaps in understanding
```bash
jeeves topic gaps <slug> --json
```

Always start with `glance` for triage, escalate to `brief` or `detailed`
only when the user needs depth. Use `--topic` to accumulate knowledge
across related queries.
