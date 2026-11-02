
  ### Key Features

  • Zero Third-Party Dependencies: Built entirely with Python standard libraries (urllib.request, json, ipaddress, argparse).
  • Environment Variable Token Support: Reads the API token from CLOUDFLARE_API_TOKEN (with fallback to CF_API_TOKEN or CLOUDFLARE_TOKEN, and an optional --token CLI flag).
  • Core Operations: Supports create, update, delete, as well as upsert and list.
  • Smart Zone Resolution: Accepts --zone-id (hex) or --zone (domain name), and can automatically detect the parent zone if only --name is supplied.
  • Validation & Control: Validates IPv4 addresses and TTL ranges; supports Cloudflare proxy status (--proxied / --no-proxied), record comments, and --json output mode.
  ──────
  ### Usage Guide

  #### 1. Set Your Cloudflare API Token

    export CLOUDFLARE_API_TOKEN="your-cloudflare-api-token"

  #### 2. Create an A Record

    # Basic creation (auto-detects zone)
    python3 hack/cf/cf.py create --name sub.example.com --content 192.0.2.1

    # With Cloudflare proxy enabled and custom TTL
    python3 hack/cf/cf.py create --name sub.example.com --content 192.0.2.1 --proxied --ttl 1

    # Explicit zone or zone ID
    python3 hack/cf/cf.py create --name sub.example.com --content 192.0.2.1 --zone example.com

  #### 3. Update an A Record

    # Update IP address by domain name
    python3 hack/cf/cf.py update --name sub.example.com --content 192.0.2.2

    # Update by record ID directly
    python3 hack/cf/cf.py update --record-id <RECORD_ID> --content 192.0.2.2 --zone example.com

  #### 4. Delete an A Record

    # Delete by record name
    python3 hack/cf/cf.py delete --name sub.example.com

    # Delete by specific record ID
    python3 hack/cf/cf.py delete --record-id <RECORD_ID> --zone example.com

  #### 5. Upsert (Create or Update)

    # Creates the record if missing; updates if already exists
    python3 hack/cf/cf.py upsert --name sub.example.com --content 192.0.2.3

  #### 6. List Records

    python3 hack/cf/cf.py list --zone example.com
