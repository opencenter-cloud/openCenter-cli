# cluster schema


## Table of Contents

- [Synopsis](#synopsis)
- [Description](#description)
- [Flags](#flags)
- [Examples](#examples)
- [Schema Format](#schema-format)
- [Schema Structure](#schema-structure)
- [Use Cases](#use-cases)
- [Schema Version](#schema-version)
- [Output](#output)
- [See Also](#see-also)
**doc_type:** reference

Export cluster JSON schema with validation rules.

## Synopsis

```bash
openCenter cluster schema [flags]
```

## Description

The `cluster schema` command exports the JSON schema for openCenter cluster configuration. The schema includes comprehensive validation rules, constraints, and documentation for all configuration sections.

**Note:** This command is hidden from help output as it is primarily intended for internal use and IDE integration.

## Flags

- `--out string` - Output file path (default: stdout)
- `--pretty` - Pretty-print JSON output (default: true)
- `--version` - Show schema version

## Examples

```bash
# Print schema to stdout
openCenter cluster schema

# Print schema with pretty formatting
openCenter cluster schema --pretty

# Save schema to file
openCenter cluster schema --out schema/cluster.schema.json

# Save schema without pretty formatting
openCenter cluster schema --out schema/cluster.schema.json --pretty=false

# Show schema version
openCenter cluster schema --version
```

## Schema Format

The schema follows JSON Schema Draft 2020-12 format and includes:

- **Type definitions** - Data types for all fields
- **Validation rules** - Required fields, patterns, constraints
- **Default values** - Sensible defaults for optional fields
- **Descriptions** - Documentation for each field
- **Examples** - Sample values for complex fields
- **Constraints** - Min/max values, enum options, format validation

## Schema Structure

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/rackerlabs/openCenter-cli/schema/cluster.schema.json",
  "title": "openCenter Cluster Configuration",
  "description": "Complete schema for openCenter cluster configuration",
  "type": "object",
  "properties": {
    "opencenter": {
      "type": "object",
      "properties": {
        "meta": { ... },
        "cluster": { ... },
        "infrastructure": { ... },
        "gitops": { ... },
        "services": { ... }
      }
    },
    "secrets": { ... }
  },
  "required": ["opencenter"]
}
```

## Use Cases

### IDE Integration

Configure IDE for schema validation and autocomplete:

**VS Code (.vscode/settings.json):**
```json
{
  "yaml.schemas": {
    "schema/cluster.schema.json": [
      "**/*-config.yaml",
      "**/cluster-*.yaml"
    ]
  }
}
```

**JetBrains IDEs (.idea/jsonSchemas.xml):**
```xml
<project version="4">
  <component name="JsonSchemaMappingsProjectConfiguration">
    <state>
      <map>
        <entry key="openCenter Cluster">
          <value>
            <SchemaInfo>
              <option name="name" value="openCenter Cluster" />
              <option name="relativePathToSchema" value="schema/cluster.schema.json" />
              <option name="patterns">
                <list>
                  <Item>
                    <option name="pattern" value="*-config.yaml" />
                  </Item>
                </list>
              </option>
            </SchemaInfo>
          </value>
        </entry>
      </map>
    </state>
  </component>
</project>
```

### Documentation Generation

Generate documentation from schema:
```bash
# Export schema
openCenter cluster schema --out schema/cluster.schema.json

# Generate markdown documentation
npx @adobe/jsonschema2md -d schema -o docs/schema
```

### Validation Tools

Use schema for external validation:
```bash
# Export schema
openCenter cluster schema --out schema/cluster.schema.json

# Validate configuration with ajv-cli
ajv validate -s schema/cluster.schema.json -d my-cluster-config.yaml
```

### CI/CD Integration

Validate configurations in CI/CD pipelines:
```bash
#!/bin/bash
set -e

# Generate schema
openCenter cluster schema --out /tmp/cluster.schema.json

# Validate all cluster configs
for config in clusters/**/*-config.yaml; do
  echo "Validating $config"
  ajv validate -s /tmp/cluster.schema.json -d "$config"
done
```

## Schema Version

Check the current schema version:
```bash
openCenter cluster schema --version
```

Output:
```
Schema version: 1.0.0
```

## Output

### Pretty-Printed (Default)

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/rackerlabs/openCenter-cli/schema/cluster.schema.json",
  "title": "openCenter Cluster Configuration",
  "type": "object",
  "properties": {
    "opencenter": {
      "type": "object",
      "required": ["meta", "cluster", "infrastructure"],
      "properties": {
        "meta": {
          "type": "object",
          "description": "Cluster metadata",
          "properties": {
            "name": {
              "type": "string",
              "description": "Cluster display name"
            }
          }
        }
      }
    }
  }
}
```

### Compact (--pretty=false)

```json
{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://github.com/rackerlabs/openCenter-cli/schema/cluster.schema.json","title":"openCenter Cluster Configuration","type":"object","properties":{"opencenter":{"type":"object","required":["meta","cluster","infrastructure"],"properties":{"meta":{"type":"object","description":"Cluster metadata","properties":{"name":{"type":"string","description":"Cluster display name"}}}}}}}
```

## See Also

- [cluster validate](../cli-commands.md#cluster-validate) - Validate cluster configuration
- [cluster init](init.md) - Initialize cluster with schema defaults
- [config ide](../cli-commands.md#config-ide) - Generate IDE configuration files
