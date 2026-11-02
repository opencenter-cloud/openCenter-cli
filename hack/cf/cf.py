#!/usr/bin/env python3
"""
Cloudflare DNS A-Record Management Script.

Manage (create, update, delete, upsert, list) Cloudflare DNS A records using
a Cloudflare API token provided via environment variable (or CLI argument).

Supported Environment Variables:
  - CLOUDFLARE_API_TOKEN (primary)
  - CF_API_TOKEN
  - CLOUDFLARE_TOKEN

Usage Examples:
  # Export your token
  export CLOUDFLARE_API_TOKEN="your-api-token"

  # Create an A record
  python3 hack/cl.py create --name sub.example.com --content 192.0.2.1

  # Create with proxy enabled and custom TTL
  python3 hack/cl.py create --name sub.example.com --content 192.0.2.1 --proxied

  # Upsert (create if missing, update if existing)
  python3 hack/cl.py upsert --name sub.example.com --content 192.0.2.2

  # Update an existing A record by name
  python3 hack/cl.py update --name sub.example.com --content 192.0.2.3

  # Delete an A record by name
  python3 hack/cl.py delete --name sub.example.com

  # List A records for a zone
  python3 hack/cl.py list --zone example.com
"""

import argparse
import ipaddress
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request
from typing import Any, Dict, List, Optional, Tuple

API_BASE_URL = "https://api.cloudflare.com/client/v4"
ENV_TOKEN_VARS = ["CLOUDFLARE_API_TOKEN", "CF_API_TOKEN", "CLOUDFLARE_TOKEN"]


class CloudflareAPIError(Exception):
    """Custom exception for Cloudflare API errors."""

    def __init__(self, message: str, status_code: Optional[int] = None, errors: Optional[List[dict]] = None):
        super().__init__(message)
        self.status_code = status_code
        self.errors = errors or []


def get_api_token(cli_token: Optional[str] = None) -> str:
    """Retrieve Cloudflare API token from CLI argument or environment variables."""
    if cli_token:
        return cli_token.strip()

    for var in ENV_TOKEN_VARS:
        token = os.environ.get(var)
        if token and token.strip():
            return token.strip()

    print(
        "Error: Cloudflare API token not found.\n"
        "Please provide a token via --token or set one of the following environment variables:\n"
        f"  - {', '.join(ENV_TOKEN_VARS)}",
        file=sys.stderr,
    )
    sys.exit(1)


def validate_ipv4(ip_str: str) -> str:
    """Validate that the given string is a valid IPv4 address."""
    clean_ip = ip_str.strip()
    try:
        ipaddress.IPv4Address(clean_ip)
        return clean_ip
    except ipaddress.AddressValueError as err:
        raise ValueError(f"Invalid IPv4 address '{clean_ip}': {err}") from err


def validate_ttl(ttl: int) -> int:
    """Validate DNS TTL (1 = automatic, or 60 to 86400 seconds)."""
    if ttl == 1 or 60 <= ttl <= 86400:
        return ttl
    raise ValueError(f"Invalid TTL {ttl}. Must be 1 (automatic) or between 60 and 86400 seconds.")


def cf_request(
    endpoint: str,
    token: str,
    method: str = "GET",
    data: Optional[Dict[str, Any]] = None,
    debug: bool = False,
) -> Dict[str, Any]:
    """Execute an HTTP request to the Cloudflare API v4."""
    url = f"{API_BASE_URL}{endpoint}"
    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
        "Accept": "application/json",
        "User-Agent": "openCenter-cli-cf/1.0",
    }

    body = json.dumps(data).encode("utf-8") if data is not None else None

    if debug:
        print(f"[DEBUG] {method} {url}", file=sys.stderr)
        if body:
            print(f"[DEBUG] Request Body: {body.decode('utf-8')}", file=sys.stderr)

    req = urllib.request.Request(url, data=body, headers=headers, method=method)

    try:
        with urllib.request.urlopen(req) as resp:
            resp_body = resp.read().decode("utf-8")
            if debug:
                print(f"[DEBUG] Response Status: {resp.status}", file=sys.stderr)
                print(f"[DEBUG] Response Body: {resp_body}", file=sys.stderr)
            return json.loads(resp_body)
    except urllib.error.HTTPError as e:
        err_body = e.read().decode("utf-8")
        if debug:
            print(f"[DEBUG] HTTPError {e.code}: {err_body}", file=sys.stderr)
        try:
            err_json = json.loads(err_body)
            errors = err_json.get("errors", [])
            messages = [f"[code {err.get('code')}] {err.get('message')}" for err in errors]
            msg_str = "; ".join(messages) if messages else err_body
            raise CloudflareAPIError(f"Cloudflare API error (HTTP {e.code}): {msg_str}", e.code, errors) from e
        except json.JSONDecodeError:
            raise CloudflareAPIError(f"HTTP {e.code}: {err_body}", e.code) from e
    except urllib.error.URLError as e:
        raise CloudflareAPIError(f"Network error connecting to Cloudflare API: {e.reason}") from e


def get_zone_id_by_name(zone_name: str, token: str, debug: bool = False) -> Optional[str]:
    """Look up Cloudflare Zone ID by exact zone name."""
    query = urllib.parse.urlencode({"name": zone_name, "status": "active"})
    res = cf_request(f"/zones?{query}", token=token, debug=debug)
    zones = res.get("result", [])
    if zones:
        return zones[0]["id"]
    return None


def resolve_zone_id(zone_input: Optional[str], record_name: Optional[str], token: str, debug: bool = False) -> str:
    """
    Resolve the Zone ID from user input or record name.
    1. If zone_input is a 32-character hex ID, use directly.
    2. If zone_input is a domain name, look it up.
    3. If zone_input is omitted, try matching parent domains of record_name.
    """
    if zone_input:
        clean_zone = zone_input.strip()
        # 32-char hex string check (Cloudflare Zone ID format)
        if len(clean_zone) == 32 and all(c in "0123456789abcdefABCDEF" for c in clean_zone):
            return clean_zone
        # Treat as zone name
        zone_id = get_zone_id_by_name(clean_zone, token, debug=debug)
        if zone_id:
            return zone_id
        raise CloudflareAPIError(f"No active Cloudflare zone found for '{clean_zone}'.")

    if not record_name:
        raise CloudflareAPIError("Please provide either --zone or a fully qualified --name.")

    # Auto-detect zone from record_name (e.g. sub.domain.example.com -> domain.example.com -> example.com)
    clean_name = record_name.strip().rstrip(".")
    parts = clean_name.split(".")
    if len(parts) < 2:
        raise CloudflareAPIError(
            f"Cannot infer zone from name '{record_name}'. Please specify --zone or --zone-id."
        )

    # Check potential domain suffixes from shortest to longest (e.g., example.com before sub.example.com)
    for i in range(len(parts) - 1, 0, -1):
        candidate_zone = ".".join(parts[i - 1 :])
        zone_id = get_zone_id_by_name(candidate_zone, token, debug=debug)
        if zone_id:
            if debug:
                print(f"[DEBUG] Inferred zone '{candidate_zone}' (ID: {zone_id}) from '{record_name}'", file=sys.stderr)
            return zone_id

    raise CloudflareAPIError(
        f"Could not automatically determine Cloudflare Zone for record '{record_name}'. "
        "Please specify --zone or --zone-id."
    )


def list_a_records(
    zone_id: str,
    token: str,
    name: Optional[str] = None,
    debug: bool = False,
) -> List[Dict[str, Any]]:
    """List A records in a zone, optionally filtered by name."""
    params: Dict[str, str] = {"type": "A", "per_page": "100"}
    if name:
        params["name"] = name.strip()
    query = urllib.parse.urlencode(params)
    res = cf_request(f"/zones/{zone_id}/dns_records?{query}", token=token, debug=debug)
    return res.get("result", [])


def create_a_record(
    zone_id: str,
    token: str,
    name: str,
    content: str,
    ttl: int = 1,
    proxied: bool = False,
    comment: Optional[str] = None,
    debug: bool = False,
) -> Dict[str, Any]:
    """Create a new DNS A record."""
    validated_ip = validate_ipv4(content)
    validated_ttl = validate_ttl(ttl)

    payload: Dict[str, Any] = {
        "type": "A",
        "name": name.strip(),
        "content": validated_ip,
        "ttl": validated_ttl,
        "proxied": bool(proxied),
    }
    if comment:
        payload["comment"] = comment.strip()

    return cf_request(f"/zones/{zone_id}/dns_records", token=token, method="POST", data=payload, debug=debug)


def update_a_record(
    zone_id: str,
    token: str,
    record_id: str,
    name: Optional[str] = None,
    content: Optional[str] = None,
    ttl: Optional[int] = None,
    proxied: Optional[bool] = None,
    comment: Optional[str] = None,
    debug: bool = False,
) -> Dict[str, Any]:
    """Update an existing DNS A record using PATCH."""
    payload: Dict[str, Any] = {}
    if name:
        payload["name"] = name.strip()
    if content:
        payload["content"] = validate_ipv4(content)
    if ttl is not None:
        payload["ttl"] = validate_ttl(ttl)
    if proxied is not None:
        payload["proxied"] = bool(proxied)
    if comment is not None:
        payload["comment"] = comment.strip()

    if not payload:
        raise CloudflareAPIError("No fields specified to update.")

    return cf_request(
        f"/zones/{zone_id}/dns_records/{record_id}",
        token=token,
        method="PATCH",
        data=payload,
        debug=debug,
    )


def delete_a_record(zone_id: str, token: str, record_id: str, debug: bool = False) -> Dict[str, Any]:
    """Delete a DNS A record by record ID."""
    return cf_request(f"/zones/{zone_id}/dns_records/{record_id}", token=token, method="DELETE", debug=debug)


def handle_create(args: argparse.Namespace, token: str) -> int:
    zone_id = resolve_zone_id(args.zone or args.zone_id, args.name, token, debug=args.debug)
    res = create_a_record(
        zone_id=zone_id,
        token=token,
        name=args.name,
        content=args.content,
        ttl=args.ttl,
        proxied=args.proxied or False,
        comment=args.comment,
        debug=args.debug,
    )

    if args.json:
        print(json.dumps(res, indent=2))
    else:
        record = res.get("result", {})
        rec_id = record.get("id", "N/A")
        rec_name = record.get("name", args.name)
        rec_content = record.get("content", args.content)
        ttl_display = "auto" if record.get("ttl") == 1 else f"{record.get('ttl')}s"
        proxied_str = "proxied" if record.get("proxied") else "DNS only"
        print(f"✓ Created A record: {rec_name} -> {rec_content} [ID: {rec_id}, TTL: {ttl_display}, {proxied_str}]")
    return 0


def handle_update(args: argparse.Namespace, token: str) -> int:
    zone_id = resolve_zone_id(args.zone or args.zone_id, args.name, token, debug=args.debug)
    record_id = args.record_id

    # If record_id not provided, look it up by name
    if not record_id:
        if not args.name:
            print("Error: Either --record-id or --name must be specified for update.", file=sys.stderr)
            return 1

        records = list_a_records(zone_id, token, name=args.name, debug=args.debug)
        if not records:
            print(f"Error: No existing A record found matching '{args.name}'.", file=sys.stderr)
            print("Tip: Use 'create' or 'upsert' to create a new record.", file=sys.stderr)
            return 1

        if len(records) > 1 and not args.old_content:
            print(
                f"Error: Multiple A records found for '{args.name}'. "
                f"Please specify --record-id or disambiguate with --old-content.",
                file=sys.stderr,
            )
            for r in records:
                print(f"  - ID: {r['id']}, Content: {r['content']}", file=sys.stderr)
            return 1

        if args.old_content:
            matching = [r for r in records if r.get("content") == args.old_content.strip()]
            if not matching:
                print(f"Error: No A record found for '{args.name}' with IP '{args.old_content}'.", file=sys.stderr)
                return 1
            record_id = matching[0]["id"]
        else:
            record_id = records[0]["id"]

    res = update_a_record(
        zone_id=zone_id,
        token=token,
        record_id=record_id,
        name=args.name,
        content=args.content,
        ttl=args.ttl,
        proxied=args.proxied,
        comment=args.comment,
        debug=args.debug,
    )

    if args.json:
        print(json.dumps(res, indent=2))
    else:
        record = res.get("result", {})
        rec_id = record.get("id", record_id)
        rec_name = record.get("name", args.name or "")
        rec_content = record.get("content", args.content or "")
        ttl_display = "auto" if record.get("ttl") == 1 else f"{record.get('ttl')}s"
        proxied_str = "proxied" if record.get("proxied") else "DNS only"
        print(f"✓ Updated A record: {rec_name} -> {rec_content} [ID: {rec_id}, TTL: {ttl_display}, {proxied_str}]")
    return 0


def handle_delete(args: argparse.Namespace, token: str) -> int:
    zone_id = resolve_zone_id(args.zone or args.zone_id, args.name, token, debug=args.debug)
    record_ids_to_delete: List[Tuple[str, str, str]] = []  # (id, name, content)

    if args.record_id:
        record_ids_to_delete.append((args.record_id, args.name or "N/A", "N/A"))
    else:
        if not args.name:
            print("Error: Either --record-id or --name must be specified for delete.", file=sys.stderr)
            return 1

        records = list_a_records(zone_id, token, name=args.name, debug=args.debug)
        if not records:
            print(f"Error: No existing A record found matching '{args.name}'.", file=sys.stderr)
            return 1

        if args.content:
            records = [r for r in records if r.get("content") == args.content.strip()]
            if not records:
                print(f"Error: No A record found for '{args.name}' matching content '{args.content}'.", file=sys.stderr)
                return 1

        if len(records) > 1 and not args.all:
            print(
                f"Error: Multiple A records found for '{args.name}'. "
                f"Use --all to delete all of them, or specify --record-id or --content.",
                file=sys.stderr,
            )
            for r in records:
                print(f"  - ID: {r['id']}, Content: {r['content']}", file=sys.stderr)
            return 1

        for r in records:
            record_ids_to_delete.append((r["id"], r.get("name", args.name), r.get("content", "")))

    results = []
    for rec_id, rec_name, rec_content in record_ids_to_delete:
        res = delete_a_record(zone_id=zone_id, token=token, record_id=rec_id, debug=args.debug)
        results.append(res)
        if not args.json:
            print(f"✓ Deleted A record: {rec_name} ({rec_content}) [ID: {rec_id}]")

    if args.json:
        print(json.dumps(results if len(results) > 1 else results[0], indent=2))
    return 0


def handle_upsert(args: argparse.Namespace, token: str) -> int:
    """Create record if it doesn't exist, update it if it does."""
    zone_id = resolve_zone_id(args.zone or args.zone_id, args.name, token, debug=args.debug)
    records = list_a_records(zone_id, token, name=args.name, debug=args.debug)

    if not records:
        # Create
        res = create_a_record(
            zone_id=zone_id,
            token=token,
            name=args.name,
            content=args.content,
            ttl=args.ttl,
            proxied=args.proxied or False,
            comment=args.comment,
            debug=args.debug,
        )
        action_verb = "Created"
    else:
        # Update first matching record
        record_id = records[0]["id"]
        res = update_a_record(
            zone_id=zone_id,
            token=token,
            record_id=record_id,
            name=args.name,
            content=args.content,
            ttl=args.ttl,
            proxied=args.proxied,
            comment=args.comment,
            debug=args.debug,
        )
        action_verb = "Updated"

    if args.json:
        print(json.dumps(res, indent=2))
    else:
        record = res.get("result", {})
        rec_id = record.get("id", "N/A")
        rec_name = record.get("name", args.name)
        rec_content = record.get("content", args.content)
        ttl_display = "auto" if record.get("ttl") == 1 else f"{record.get('ttl')}s"
        proxied_str = "proxied" if record.get("proxied") else "DNS only"
        print(f"✓ {action_verb} A record: {rec_name} -> {rec_content} [ID: {rec_id}, TTL: {ttl_display}, {proxied_str}]")
    return 0


def handle_list(args: argparse.Namespace, token: str) -> int:
    zone_id = resolve_zone_id(args.zone or args.zone_id, args.name, token, debug=args.debug)
    records = list_a_records(zone_id, token, name=args.name, debug=args.debug)

    if args.json:
        print(json.dumps(records, indent=2))
    else:
        if not records:
            print("No A records found.")
            return 0
        print(f"Found {len(records)} A record(s):")
        print(f"{'NAME':<35} {'CONTENT':<18} {'TTL':<8} {'PROXIED':<10} {'RECORD ID'}")
        print("-" * 105)
        for r in records:
            ttl_str = "auto" if r.get("ttl") == 1 else f"{r.get('ttl')}s"
            proxied_str = "true" if r.get("proxied") else "false"
            print(f"{r.get('name', ''):<35} {r.get('content', ''):<18} {ttl_str:<8} {proxied_str:<10} {r.get('id', '')}")
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Manage Cloudflare DNS A records using the Cloudflare API v4.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Environment Variables:
  CLOUDFLARE_API_TOKEN   API Token with Zone.DNS permissions (recommended)
  CF_API_TOKEN           Alternative token env var
  CLOUDFLARE_TOKEN       Alternative token env var
""",
    )

    # Global options
    parser.add_argument(
        "--token",
        "-t",
        help="Cloudflare API Token (overrides CLOUDFLARE_API_TOKEN env var)",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Output raw JSON response from Cloudflare API",
    )
    parser.add_argument(
        "--debug",
        action="store_true",
        help="Enable debug logging of HTTP requests and responses",
    )

    subparsers = parser.add_subparsers(dest="command", required=True, help="Command to execute")

    # Common zone arguments helper
    def add_zone_args(subparser: argparse.ArgumentParser) -> None:
        subparser.add_argument(
            "--zone",
            "-z",
            help="Zone name (e.g. example.com) or Zone ID. Auto-detected from record name if omitted.",
        )
        subparser.add_argument(
            "--zone-id",
            help="Cloudflare Zone ID (32 hex characters).",
        )

    # Subcommand: create
    parser_create = subparsers.add_parser("create", help="Create a new A record")
    parser_create.add_argument("--name", "-n", required=True, help="Record name (e.g. sub.example.com or @)")
    parser_create.add_argument("--content", "--ip", "-c", required=True, help="IPv4 address content")
    parser_create.add_argument(
        "--ttl",
        type=int,
        default=1,
        help="TTL in seconds (1 for auto, or 60-86400). Default: 1 (auto)",
    )
    parser_create.add_argument(
        "--proxied",
        action=argparse.BooleanOptionalAction,
        default=False,
        help="Proxy traffic through Cloudflare (default: --no-proxied)",
    )
    parser_create.add_argument("--comment", help="Optional record comment")
    add_zone_args(parser_create)
    parser_create.set_defaults(func=handle_create)

    # Subcommand: update
    parser_update = subparsers.add_parser("update", help="Update an existing A record")
    parser_update.add_argument("--name", "-n", help="Record name (e.g. sub.example.com)")
    parser_update.add_argument("--record-id", "-r", help="Specific DNS record ID")
    parser_update.add_argument("--content", "--ip", "-c", help="New IPv4 address content")
    parser_update.add_argument(
        "--old-content",
        help="Disambiguate if multiple A records share the same name by matching existing IP",
    )
    parser_update.add_argument(
        "--ttl",
        type=int,
        help="TTL in seconds (1 for auto, or 60-86400)",
    )
    parser_update.add_argument(
        "--proxied",
        action=argparse.BooleanOptionalAction,
        default=None,
        help="Enable or disable Cloudflare proxying",
    )
    parser_update.add_argument("--comment", help="New record comment")
    add_zone_args(parser_update)
    parser_update.set_defaults(func=handle_update)

    # Subcommand: delete
    parser_delete = subparsers.add_parser("delete", help="Delete an A record")
    parser_delete.add_argument("--name", "-n", help="Record name to delete")
    parser_delete.add_argument("--record-id", "-r", help="Specific DNS record ID to delete")
    parser_delete.add_argument("--content", "--ip", "-c", help="Filter by specific IP address if multiple exist")
    parser_delete.add_argument(
        "--all",
        action="store_true",
        help="Delete all matching A records if multiple exist for the name",
    )
    add_zone_args(parser_delete)
    parser_delete.set_defaults(func=handle_delete)

    # Subcommand: upsert
    parser_upsert = subparsers.add_parser("upsert", help="Create A record if missing, update if existing")
    parser_upsert.add_argument("--name", "-n", required=True, help="Record name (e.g. sub.example.com)")
    parser_upsert.add_argument("--content", "--ip", "-c", required=True, help="IPv4 address content")
    parser_upsert.add_argument(
        "--ttl",
        type=int,
        default=1,
        help="TTL in seconds (1 for auto, or 60-86400). Default: 1",
    )
    parser_upsert.add_argument(
        "--proxied",
        action=argparse.BooleanOptionalAction,
        default=False,
        help="Proxy traffic through Cloudflare (default: --no-proxied)",
    )
    parser_upsert.add_argument("--comment", help="Optional record comment")
    add_zone_args(parser_upsert)
    parser_upsert.set_defaults(func=handle_upsert)

    # Subcommand: list
    parser_list = subparsers.add_parser("list", help="List A records in a zone")
    parser_list.add_argument("--name", "-n", help="Filter records by name")
    add_zone_args(parser_list)
    parser_list.set_defaults(func=handle_list)

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()

    token = get_api_token(args.token)

    try:
        return args.func(args, token)
    except (CloudflareAPIError, ValueError) as e:
        print(f"Error: {e}", file=sys.stderr)
        return 1
    except KeyboardInterrupt:
        print("\nAborted.", file=sys.stderr)
        return 130


if __name__ == "__main__":
    sys.exit(main())
