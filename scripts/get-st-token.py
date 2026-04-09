#!/usr/bin/env python3
"""Fetch a bearer token from Samsung SmartThings using OAuth2 authorization code flow.

Opens a browser for Samsung SSO login, listens for the callback, exchanges
the code, and prints the access token. Optionally tests the per-device
preferences endpoint to see if the OAuth token has more access than a PAT.

Setup:
  1. Install the SmartThings CLI:  npm install -g @smartthings/cli
  2. Create an OAuth app:          smartthings apps:create
     - Choose "OAuth-In App"
     - Add redirect URI: http://localhost:8401/callback
     - Select ALL scopes (r:devices:*, w:devices:*, x:devices:*, etc.)
     - Save the client_id and client_secret
  3. Add to your .env file:
       ST_OAUTH_CLIENT_ID=<your-client-id>
       ST_OAUTH_CLIENT_SECRET=<your-client-secret>
  4. Run:  python3 scripts/get-st-token.py [--test-prefs DEVICE_ID]

Requires: pip install requests
"""

import argparse
import base64
import http.server
import json
import os
import secrets
import sys
import urllib.parse
import webbrowser

import requests

# SmartThings OAuth endpoints
ST_AUTH_URL = "https://api.smartthings.com/oauth/authorize"
ST_TOKEN_URL = "https://auth-global.api.smartthings.com/oauth/token"
ST_API_BASE = "https://api.smartthings.com"

CALLBACK_PORT = 8401
REDIRECT_URI = f"http://localhost:{CALLBACK_PORT}/callback"

# Scopes validated as accepted by SmartThings OAuth app creation.
# Invalid scopes (w:hubs:*, w:deviceprofiles, r/w:drivers:*, r/w:channels:*,
# w:customcapability) are rejected by the API.
ALL_SCOPES = [
    "r:devices:*", "w:devices:*", "x:devices:*",
    "r:hubs:*",
    "r:locations:*", "w:locations:*", "x:locations:*",
    "r:scenes:*", "x:scenes:*",
    "r:rules:*", "w:rules:*",
    "r:installedapps", "w:installedapps",
    "r:deviceprofiles",
    "r:customcapability",
]


def load_env(path):
    """Load key=value pairs from a .env file."""
    if not os.path.exists(path):
        return
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            if "=" in line:
                key, _, value = line.partition("=")
                os.environ.setdefault(key.strip(), value.strip())


class CallbackHandler(http.server.BaseHTTPRequestHandler):
    """Handles the OAuth callback and captures the authorization code."""

    def do_GET(self):
        qs = urllib.parse.parse_qs(urllib.parse.urlparse(self.path).query)
        self.server.auth_code = qs.get("code", [None])[0]
        self.server.auth_error = qs.get("error_description", qs.get("error", [None]))[0]

        if self.server.auth_code:
            body = b"<html><body><h2>SmartThings login successful! You can close this tab.</h2></body></html>"
            self.send_response(200)
        else:
            body = f"<html><body><h2>Login failed: {self.server.auth_error}</h2></body></html>".encode()
            self.send_response(400)

        self.send_header("Content-Type", "text/html")
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


def exchange_code(client_id, client_secret, code):
    """Exchange authorization code for access + refresh tokens."""
    auth = base64.b64encode(f"{client_id}:{client_secret}".encode()).decode()
    resp = requests.post(
        ST_TOKEN_URL,
        headers={
            "Authorization": f"Basic {auth}",
            "Content-Type": "application/x-www-form-urlencoded",
        },
        data={
            "grant_type": "authorization_code",
            "code": code,
            "redirect_uri": REDIRECT_URI,
            "client_id": client_id,
        },
        timeout=30,
    )
    resp.raise_for_status()
    return resp.json()


def test_preferences(token, device_id):
    """Test GET and PUT on preferences endpoint with the OAuth token."""
    headers = {
        "Authorization": f"Bearer {token}",
    }
    print("\n--- Testing preferences endpoint with OAuth token ---", file=sys.stderr)

    # Test GET with various Accept headers
    for accept in [
        "application/json",
        "application/vnd.smartthings+json;v=20170916",
        "application/vnd.smartthings+json;v=20250122",
    ]:
        resp = requests.get(
            f"{ST_API_BASE}/v1/devices/{device_id}/preferences",
            headers={**headers, "Accept": accept},
            timeout=10,
        )
        print(f"  GET  /v1/devices/.../preferences  Accept: {accept.split(';')[0]:40s}  → {resp.status_code}", file=sys.stderr)
        if resp.status_code == 200:
            print(f"  *** SUCCESS! Response: {resp.text[:500]}", file=sys.stderr)
            return True

    # Test without /v1/ prefix
    resp = requests.get(
        f"{ST_API_BASE}/devices/{device_id}/preferences",
        headers={**headers, "Accept": "application/vnd.smartthings+json;v=20170916"},
        timeout=10,
    )
    print(f"  GET  /devices/.../preferences (no /v1/)                               → {resp.status_code}", file=sys.stderr)
    if resp.status_code == 200:
        print(f"  *** SUCCESS! Response: {resp.text[:500]}", file=sys.stderr)
        return True

    # Test PUT
    resp = requests.put(
        f"{ST_API_BASE}/v1/devices/{device_id}/preferences",
        headers={**headers, "Accept": "application/vnd.smartthings+json;v=20170916", "Content-Type": "application/json"},
        json={"parameter112": "1"},
        timeout=10,
    )
    print(f"  PUT  /v1/devices/.../preferences                                      → {resp.status_code}", file=sys.stderr)
    if resp.status_code == 200:
        print(f"  *** SUCCESS! Response: {resp.text[:500]}", file=sys.stderr)
        return True

    # Verify the token works at all
    resp = requests.get(
        f"{ST_API_BASE}/v1/devices/{device_id}/status",
        headers={**headers, "Accept": "application/json"},
        timeout=10,
    )
    print(f"  GET  /v1/devices/.../status (control)                                 → {resp.status_code}", file=sys.stderr)

    print("\n  Preferences endpoint still returns 406 with OAuth token.", file=sys.stderr)
    return False


def main():
    parser = argparse.ArgumentParser(
        description="Fetch a SmartThings OAuth bearer token using Samsung SSO."
    )
    parser.add_argument("--env", default=".env", help="Path to .env file (default: .env)")
    parser.add_argument("--test-prefs", metavar="DEVICE_ID",
                        help="Test the preferences endpoint with the new token")
    parser.add_argument("--scopes", default=None,
                        help="Space-separated scopes (default: all known scopes)")
    args = parser.parse_args()

    load_env(args.env)

    client_id = os.environ.get("ST_OAUTH_CLIENT_ID", "")
    client_secret = os.environ.get("ST_OAUTH_CLIENT_SECRET", "")

    if not client_id or not client_secret:
        print("Error: ST_OAUTH_CLIENT_ID and ST_OAUTH_CLIENT_SECRET must be set.", file=sys.stderr)
        print("\nTo create SmartThings OAuth credentials:", file=sys.stderr)
        print("  1. npm install -g @smartthings/cli", file=sys.stderr)
        print("  2. smartthings apps:create", file=sys.stderr)
        print("  3. Add to .env: ST_OAUTH_CLIENT_ID=... ST_OAUTH_CLIENT_SECRET=...", file=sys.stderr)
        sys.exit(1)

    scopes = args.scopes.split() if args.scopes else ALL_SCOPES
    state = secrets.token_urlsafe(32)

    # Build authorization URL
    params = {
        "client_id": client_id,
        "response_type": "code",
        "redirect_uri": REDIRECT_URI,
        "scope": " ".join(scopes),
        "state": state,
    }
    auth_url = f"{ST_AUTH_URL}?{urllib.parse.urlencode(params)}"

    # Start callback server
    srv = http.server.HTTPServer(("127.0.0.1", CALLBACK_PORT), CallbackHandler)
    srv.auth_code = None
    srv.auth_error = None
    srv.timeout = 120

    print("Opening browser for Samsung SmartThings login...", file=sys.stderr)
    print(f"If the browser doesn't open, visit:\n  {auth_url}\n", file=sys.stderr)
    webbrowser.open(auth_url)

    print("Waiting for login callback...", file=sys.stderr)
    srv.handle_request()

    if not srv.auth_code:
        error = srv.auth_error or "No authorization code received"
        print(f"Error: {error}", file=sys.stderr)
        sys.exit(1)

    # Exchange code for token
    print("Exchanging code for token...", file=sys.stderr)
    token_data = exchange_code(client_id, client_secret, srv.auth_code)

    access_token = token_data.get("access_token")
    refresh_token = token_data.get("refresh_token")
    expires_in = token_data.get("expires_in")
    granted_scopes = token_data.get("scope", "")

    if not access_token:
        print(f"Error: No access_token in response: {json.dumps(token_data)}", file=sys.stderr)
        sys.exit(1)

    print(f"\nToken type:     {token_data.get('token_type', '?')}", file=sys.stderr)
    print(f"Expires in:     {expires_in}s ({expires_in // 3600}h)" if expires_in else "", file=sys.stderr)
    print(f"Refresh token:  {'yes' if refresh_token else 'no'}", file=sys.stderr)
    print(f"Granted scopes: {granted_scopes}", file=sys.stderr)

    # Test preferences if requested
    if args.test_prefs:
        test_preferences(access_token, args.test_prefs)

    # Print token to stdout
    print(access_token)


if __name__ == "__main__":
    main()
