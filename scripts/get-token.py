#!/usr/bin/env python3
"""Fetch a bearer token from an OIDC provider using authorization code + PKCE.

Opens a browser for login, listens for the callback, exchanges the code,
and prints the access token to stdout.

Usage: python3 scripts/get-token.py [-jwt] [.env file]
Requires: pip install requests-oauthlib
"""

import argparse
import hashlib
import http.server
import json
import os
import secrets
import sys
import threading
import urllib.parse
import webbrowser
import base64

import os as _os
_os.environ["OAUTHLIB_RELAX_TOKEN_SCOPE"] = "1"

import requests
from requests_oauthlib import OAuth2Session

CALLBACK_PORT = 8400
REDIRECT_URI = f"http://localhost:{CALLBACK_PORT}/callback"


def load_env(path):
    """Load key=value pairs from a .env file."""
    if not os.path.exists(path):
        print(f"Error: {path} not found", file=sys.stderr)
        sys.exit(1)

    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            if "=" in line:
                key, _, value = line.partition("=")
                os.environ.setdefault(key.strip(), value.strip())


def discover(issuer_url):
    """Fetch OIDC discovery document."""
    url = f"{issuer_url.rstrip('/')}/.well-known/openid-configuration"
    resp = requests.get(url, timeout=10)
    resp.raise_for_status()
    return resp.json()


def decode_jwt(token_str):
    """Decode a JWT and return (header, payload) as dicts. Does not verify the signature."""
    parts = token_str.split(".")
    if len(parts) != 3:
        print("Warning: token does not look like a JWT (expected 3 dot-separated parts)", file=sys.stderr)
        return None, None

    def _b64decode(s):
        # Add padding if needed
        s += "=" * (-len(s) % 4)
        return json.loads(base64.urlsafe_b64decode(s))

    try:
        header = _b64decode(parts[0])
        payload = _b64decode(parts[1])
        return header, payload
    except Exception as e:
        print(f"Warning: failed to decode JWT: {e}", file=sys.stderr)
        return None, None


def print_jwt_info(token_str):
    """Pretty-print JWT header and payload claims."""
    header, payload = decode_jwt(token_str)
    if header is None:
        return

    print("\n--- JWT Header ---", file=sys.stderr)
    print(json.dumps(header, indent=2), file=sys.stderr)
    print("\n--- JWT Payload ---", file=sys.stderr)
    print(json.dumps(payload, indent=2), file=sys.stderr)

    # Show expiry info if present
    import datetime
    if "exp" in payload:
        exp = datetime.datetime.fromtimestamp(payload["exp"], tz=datetime.timezone.utc)
        now = datetime.datetime.now(tz=datetime.timezone.utc)
        remaining = exp - now
        print(f"\nExpires: {exp.isoformat()} (in {remaining})", file=sys.stderr)
    if "iat" in payload:
        iat = datetime.datetime.fromtimestamp(payload["iat"], tz=datetime.timezone.utc)
        print(f"Issued:  {iat.isoformat()}", file=sys.stderr)
    if "sub" in payload:
        print(f"Subject: {payload['sub']}", file=sys.stderr)
    if "iss" in payload:
        print(f"Issuer:  {payload['iss']}", file=sys.stderr)
    print(file=sys.stderr)


def generate_pkce():
    """Generate PKCE code verifier and challenge."""
    verifier = secrets.token_urlsafe(64)
    digest = hashlib.sha256(verifier.encode()).digest()
    challenge = base64.urlsafe_b64encode(digest).rstrip(b"=").decode()
    return verifier, challenge


class CallbackHandler(http.server.BaseHTTPRequestHandler):
    """Handles the OAuth callback and captures the authorization code."""

    def do_GET(self):
        qs = urllib.parse.parse_qs(urllib.parse.urlparse(self.path).query)
        self.server.auth_code = qs.get("code", [None])[0]
        self.server.auth_state = qs.get("state", [None])[0]
        self.server.auth_error = qs.get("error_description", qs.get("error", [None]))[0]

        if self.server.auth_code:
            body = b"<html><body><h2>Login successful! You can close this tab.</h2></body></html>"
            self.send_response(200)
        else:
            body = b"<html><body><h2>Login failed.</h2></body></html>"
            self.send_response(400)

        self.send_header("Content-Type", "text/html")
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass  # Suppress request logging


def main():
    parser = argparse.ArgumentParser(
        description="Fetch a bearer token from an OIDC provider using authorization code + PKCE."
    )
    parser.add_argument("-jwt", action="store_true", help="Decode and display JWT header and payload claims")
    parser.add_argument("env_file", nargs="?", default=".env", help="Path to .env file (default: .env)")
    args = parser.parse_args()

    load_env(args.env_file)

    issuer_url = os.environ.get("MCP_AUTH_OIDC_ISSUER_URL", "").rstrip("/")
    client_id = os.environ.get("MCP_AUTH_AUDIENCE", "")
    client_secret = os.environ.get("MCP_AUTH_CLIENT_SECRET", "")

    if not issuer_url:
        print("Error: MCP_AUTH_OIDC_ISSUER_URL not set", file=sys.stderr)
        sys.exit(1)
    if not client_id:
        print("Error: MCP_AUTH_AUDIENCE not set", file=sys.stderr)
        sys.exit(1)

    # Discover endpoints.
    print("Discovering OIDC endpoints...", file=sys.stderr)
    config = discover(issuer_url)
    auth_endpoint = config["authorization_endpoint"]
    token_endpoint = config["token_endpoint"]

    # Generate PKCE.
    code_verifier, code_challenge = generate_pkce()

    # Build authorization URL.
    oauth = OAuth2Session(client_id, redirect_uri=REDIRECT_URI, scope=["openid", "profile", "email", "groups"])
    auth_url, state = oauth.authorization_url(
        auth_endpoint,
        code_challenge=code_challenge,
        code_challenge_method="S256",
    )

    # Start callback server in background.
    srv = http.server.HTTPServer(("127.0.0.1", CALLBACK_PORT), CallbackHandler)
    srv.auth_code = None
    srv.auth_state = None
    srv.auth_error = None
    srv.timeout = 120

    # Open browser.
    print(f"Opening browser for login...", file=sys.stderr)
    print(f"If the browser doesn't open, visit:\n  {auth_url}\n", file=sys.stderr)
    webbrowser.open(auth_url)

    # Wait for callback.
    print("Waiting for login callback...", file=sys.stderr)
    srv.handle_request()

    if not srv.auth_code:
        error = srv.auth_error or "No authorization code received"
        print(f"Error: {error}", file=sys.stderr)
        sys.exit(1)

    # Exchange code for token.
    print("Exchanging code for token...", file=sys.stderr)
    fetch_kwargs = dict(
        code=srv.auth_code,
        code_verifier=code_verifier,
        client_id=client_id,
    )
    if client_secret:
        fetch_kwargs["client_secret"] = client_secret
    token = oauth.fetch_token(token_endpoint, **fetch_kwargs)

    access_token = token.get("access_token")
    if not access_token:
        print(f"Error: No access_token in response: {token}", file=sys.stderr)
        sys.exit(1)

    if args.jwt:
        print_jwt_info(access_token)

    print(access_token)


if __name__ == "__main__":
    main()
