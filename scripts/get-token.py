#!/usr/bin/env python3
"""Fetch a bearer token from an OIDC provider using authorization code + PKCE.

Opens a browser for login, listens for the callback, exchanges the code,
and prints the access token to stdout.

Usage: python3 scripts/get-token.py [.env file]
Requires: pip install requests-oauthlib
"""

import hashlib
import http.server
import os
import secrets
import sys
import threading
import urllib.parse
import webbrowser
import base64

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
    env_file = sys.argv[1] if len(sys.argv) > 1 else ".env"
    load_env(env_file)

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
    oauth = OAuth2Session(client_id, redirect_uri=REDIRECT_URI, scope=["openid"])
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

    print(access_token)


if __name__ == "__main__":
    main()
