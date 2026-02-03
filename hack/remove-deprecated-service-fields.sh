#!/usr/bin/env bash
# Script to remove deprecated service fields from config files

set -euo pipefail

echo "Removing deprecated service fields from config files..."

# Find all YAML config files
CONFIG_FILES=$(find cmd/clusters -name "*.yaml" -type f)

for file in $CONFIG_FILES; do
    echo "Processing: $file"
    
    # Create a temporary file
    tmpfile=$(mktemp)
    
    # Remove deprecated VSphere CSI fields
    sed -e '/^[[:space:]]*datastore_name:/d' \
        -e '/^[[:space:]]*datastoreurl:/d' \
        -e '/^[[:space:]]*delete_datastore_uuid:/d' \
        -e '/^[[:space:]]*retain_datastore_name:/d' \
        -e '/^[[:space:]]*retain_datastore_uuid:/d' \
        "$file" > "$tmpfile"
    
    # Remove deprecated Loki Swift fields
    sed -i.bak \
        -e '/^[[:space:]]*swift_username:/d' \
        -e '/^[[:space:]]*swift_project_name:/d' \
        -e '/^[[:space:]]*swift_password:/d' \
        "$tmpfile"
    
    # Move the cleaned file back
    mv "$tmpfile" "$file"
    
    # Remove backup file if it exists
    rm -f "${tmpfile}.bak"
    
    echo "  ✓ Cleaned $file"
done

echo ""
echo "✓ All config files cleaned successfully!"
echo ""
echo "Deprecated fields removed:"
echo "  - VSphere CSI: datastore_name, datastoreurl, delete_datastore_uuid, retain_datastore_name, retain_datastore_uuid"
echo "  - Loki: swift_username, swift_project_name, swift_password"
