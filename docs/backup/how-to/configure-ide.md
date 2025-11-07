# How-To: Configure Your IDE for Schema Validation

The `openCenter` cluster configuration is a rich YAML file with many nested fields and specific rules. To make editing this file easier and less error-prone, `openCenter` can generate a JSON Schema that describes the valid structure of the configuration.

By associating this schema with your YAML files, your IDE can provide:

*   **Real-time validation**: Get immediate feedback if a field is misspelled, misplaced, or has the wrong data type.
*   **Autocompletion**: Your editor can suggest valid field names as you type.
*   **Documentation on hover**: See descriptions of fields directly in your editor.

## Who is this for?

*   **Platform engineers** and **SREs** who regularly author or edit `openCenter` cluster YAML files.

## What you'll achieve

*   Generate a JSON Schema file for the cluster configuration.
*   Configure your IDE (e.g., Visual Studio Code) to use the schema for YAML validation.

---

### Step 1: Generate the JSON Schema

The `openCenter` CLI has a built-in command to generate the schema. It's best practice to output this to a file within your project so your IDE can easily find it.

```bash
# Ensure the schema directory exists
mkdir -p schema

# Generate the schema
./openCenter cluster schema --out schema/cluster.schema.json
```

This command creates a file named `cluster.schema.json` inside a `schema` directory. This file contains the complete structural definition of a valid `openCenter` cluster configuration.

### Step 2: Configure Your IDE

Most modern text editors and IDEs that support YAML also support JSON Schema validation. The process generally involves telling the editor to apply a specific schema file to files matching a certain pattern.

#### Example: Visual Studio Code

If you use VS Code, you can configure this in your workspace or user `settings.json`.

1.  Open your project in VS Code.
2.  Create a `.vscode` directory if one doesn't already exist.
3.  Create or open the `.vscode/settings.json` file.
4.  Add the following configuration. This tells the YAML language server (provided by the popular [YAML extension by Red Hat](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml)) to apply our schema to any files in the `~/.config/openCenter/` directory.

    ```json
    // .vscode/settings.json
    {
      "yaml.schemas": {
        "./schema/cluster.schema.json": [
          "~/.config/openCenter/*.yaml"
        ]
      }
    }
    ```
    *Note: The path to the schema is relative to the project root. The file glob pattern points to the default location where `openCenter` stores its configurations.*

5.  Save the file.

### Step 3: Verify the Integration

Now, when you open a cluster configuration file (e.g., `~/.config/openCenter/demo.yaml`), you should see the IDE assistance in action:

*   **Errors**: If you add an invalid field, it will be underlined with a red squiggle. Hovering over it will show an error message like "Property `invalid_field` is not allowed."
*   **Autocompletion**: When you start typing a new field, your editor should suggest valid options (e.g., `gitops`, `kubernetes`, `cloud`).
*   **Documentation**: Hovering over a valid field may show its type and description if available in the schema.

This simple setup can significantly improve your productivity and reduce the likelihood of configuration errors.
