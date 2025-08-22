# Raid Schema Specifications

This directory contains JSON Schema definitions for Raid configuration files.

## Schema Files

- `raid-profile.schema.json` - Schema for raid profile configuration files
- `raid-repo.schema.json` - Schema for individual repository configuration files

## Schema Version

These schemas follow the **JSON Schema Draft 2020-12** specification and are compatible with the `github.com/santhosh-tekuri/jsonschema/v6` library (v6.0.2).

The `$schema` field is properly supported and the library fully validates against the JSON Schema Draft 2020-12 specification.

## Usage

The schemas are used to validate YAML and JSON configuration files in the Raid CLI tool. The validation ensures that configuration files have the correct structure and required fields.

### Supported File Formats

- YAML files (`.yaml`, `.yml`)
- JSON files (`.json`)

### Validation

Profile files are validated against `raid-profile.schema.json` when using the `raid profile add` command. The validation checks:

- Required fields are present
- Data types are correct
- Structure matches the schema definition

## Schema Structure

### Raid Profile Schema

A raid profile configuration must contain:

- `name` (string, required) - The name of the raid profile
- `repositories` (array, optional) - Array of repository configurations
  - Each repository must have:
    - `name` (string, required) - The name of the repository
    - `path` (string, required) - The local path to the repository
    - `url` (string, required) - The URL of the repository

### Raid Repository Schema

A repository configuration must contain:

- `name` (string, required) - The name of the repository
- `branch` (string, required) - The branch to checkout
