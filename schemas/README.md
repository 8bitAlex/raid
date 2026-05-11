# Raid Schema Specifications

This directory contains the JSON Schema definitions for Raid configuration files.
It is the **single source of truth** for the schemas — files here are both:

1. Embedded into the Go binary via `go:embed` (used at runtime to validate
   profile and repo files), and
2. Copied into `site/static/schema/v1/` by `site/plugins/copy-schemas.ts` at
   docsite build time, so they are served at the canonical URLs:

   - https://raidcli.dev/schema/v1/raid-defs.schema.json
   - https://raidcli.dev/schema/v1/raid-profile.schema.json
   - https://raidcli.dev/schema/v1/raid-repo.schema.json

## Schema Files

- `raid-defs.schema.json` — Shared `$defs` (task types, environments,
  commands, install). Referenced from the other two schemas.
- `raid-profile.schema.json` — Profile file schema (`*.raid.yaml`).
- `raid-repo.schema.json` — Per-repository schema (`raid.yaml`).

## Versioning

The `/schema/v1/` path is a **public stability contract**.

- Additive changes (new optional fields, new task types, new enum values) are
  allowed within v1.
- Breaking changes (renaming or removing a field, tightening a `required` list,
  removing an enum value, turning an optional field required) require a new
  major version. Cut a `/schema/v2/` folder by freezing the current v1 files
  alongside the source and evolving the source separately.
- Bump the second position of `version` in `src/resources/app.properties` when
  publishing a new major schema version.

The `$id` fields are baked as absolute URLs so external validators resolve
cross-references without depending on the retrieval URL. Internal validation
(see `src/internal/lib/lib.go`) registers each embedded schema under its `$id`
and compiles by URL — no network access at runtime.

## Schema draft

These schemas follow **JSON Schema Draft 2020-12** and are validated with
[`github.com/santhosh-tekuri/jsonschema/v6`](https://github.com/santhosh-tekuri/jsonschema)
(v6.0.2).

## Supported file formats

YAML (`.yaml`, `.yml`) and JSON (`.json`).

## Usage

Profile files are validated against `raid-profile.schema.json` by `raid profile
add` and during config load. Repository `raid.yaml` files are validated against
`raid-repo.schema.json` when a repo is loaded.

Editors that consult [SchemaStore](https://www.schemastore.org/) — VS Code +
Red Hat YAML, JetBrains, Neovim `yaml-language-server`, Helix — auto-associate
`*.raid.yaml` and `raid.yaml` with the published schemas, so the `# yaml-language-server`
modeline is optional.
