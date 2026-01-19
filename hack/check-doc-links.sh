#!/usr/bin/env bash
# Script to check for broken links in documentation

set -euo pipefail

DOCS_DIR="${1:-docs}"
BROKEN_LINKS=0
CHECKED_FILES=0

echo "Checking documentation links in ${DOCS_DIR}..."
echo ""

# Find all markdown files
find "$DOCS_DIR" -name "*.md" -type f | sort | while read -r file; do
    CHECKED_FILES=$((CHECKED_FILES + 1))
    
    # Extract markdown links [text](url)
    links=$(grep -o '\[[^]]*\]([^)]*)' "$file" 2>/dev/null || true)
    
    if [ -z "$links" ]; then
        continue
    fi
    
    echo "Checking: $file"
    
    echo "$links" | while read -r link; do
        # Extract URL from link
        url=$(echo "$link" | sed 's/.*](\(.*\))/\1/')
        
        # Skip external URLs (http/https)
        if echo "$url" | grep -q '^https\?://'; then
            continue
        fi
        
        # Skip mailto links
        if echo "$url" | grep -q '^mailto:'; then
            continue
        fi
        
        # Skip anchors only
        if echo "$url" | grep -q '^#'; then
            continue
        fi
        
        # Get directory of current file
        file_dir=$(dirname "$file")
        
        # Resolve relative path
        if echo "$url" | grep -q '^/'; then
            # Absolute path from repo root
            target_path="${url#/}"
        else
            # Relative path from current file
            target_path="$file_dir/$url"
        fi
        
        # Remove anchor if present
        target_path="${target_path%%#*}"
        
        # Check if file or directory exists
        if [ ! -f "$target_path" ] && [ ! -d "$target_path" ]; then
            echo "  ✗ Broken link: $url"
            echo "    Expected: $target_path"
            BROKEN_LINKS=$((BROKEN_LINKS + 1))
        fi
    done
    echo ""
done

echo "================================"
echo "Link Check Complete"
echo "================================"
echo "Files checked: $CHECKED_FILES"

if [ $BROKEN_LINKS -eq 0 ]; then
    echo "✓ All links are valid!"
    exit 0
else
    echo "✗ Found broken links"
    exit 1
fi
