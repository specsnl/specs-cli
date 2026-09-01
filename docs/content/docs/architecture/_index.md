---
title: Architecture
weight: 10
prev: /docs/storage
next: /docs/architecture/overview
---

Internal design documents covering the architecture, key decisions, and engine internals of the Specs CLI.

{{< cards >}}
  {{< card link="overview" title="Overview" subtitle="Package structure, CLI command tree, data flows, and a summary of changes from boilr v1." >}}
  {{< card link="template-engine" title="Template Engine" subtitle="How templates are structured, rendered, and validated — including conditional files and hooks." >}}
  {{< card link="computed-values" title="Computed Values" subtitle="Post-prompt derived context keys: syntax, resolution order, and dependency handling." >}}
  {{< card link="library-decisions" title="Library Decisions" subtitle="Rationale behind every third-party dependency chosen for the CLI." >}}
  {{< card link="output" title="Output" subtitle="The stdout/stderr contract, the Writer interface, colour decisions, and golden-file tests." >}}
{{< /cards >}}
