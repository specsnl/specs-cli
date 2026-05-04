---
title: Specs CLI
layout: hextra-home
---

<div class="hx-mt-6 hx-mb-6">
{{< hextra/hero-headline >}}
  Scaffold projects from templates
{{< /hextra/hero-headline >}}
</div>

<div style="margin-bottom: 3rem">
{{< hextra/hero-subtitle >}}
  A general-purpose developer CLI for scaffolding projects from templates.
  Define variables, write template files, run hooks — **specs** handles the rest.
{{< /hextra/hero-subtitle >}}
</div>

<div style="margin-bottom: 4rem">
{{< hextra/hero-button text="Get Started" link="docs/installation" >}}
{{< hextra/hero-button text="GitHub" link="https://github.com/specsnl/specs-cli" style="outline" >}}
</div>

<div class="hx-mt-6"></div>

{{< hextra/feature-grid >}}
  {{< hextra/feature-card
    title="Template Scaffolding"
    subtitle="Define variables with automatic type inference. Use standard Go template syntax — or custom delimiters to avoid conflicts."
  >}}
  {{< hextra/feature-card
    title="Local Registry"
    subtitle="Download and manage templates locally. Reuse them any time with `specs template use`."
  >}}
  {{< hextra/feature-card
    title="Lifecycle Hooks"
    subtitle="Run pre/post-use shell commands or scripts with full access to all template variables."
  >}}
  {{< hextra/feature-card
    title="200+ Template Functions"
    subtitle="String manipulation, encoding, crypto, date/time, and more via the Sprout library."
  >}}
  {{< hextra/feature-card
    title="Flexible Sources"
    subtitle="Use templates from GitHub, HTTPS URLs, SSH, or local paths — no configuration needed."
  >}}
  {{< hextra/feature-card
    title="No Local Go Required"
    subtitle="Build and test using Docker and Task. No local toolchain installation needed."
  >}}
{{< /hextra/feature-grid >}}
