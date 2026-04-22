#!/usr/bin/env python3
"""
Run a local end-to-end API flow:

1. Register a random user.
2. Log in and read the returned user access token.
3. Query the user's balance/profile.
4. Create an unlimited default-group API token.
5. Fetch the full API key for that token.
6. Query user-available models.
7. Use the first model to call /v1/chat/completions once.

The script uses only Python's standard library.
"""

from __future__ import annotations

import argparse
import json
import os
import random
import string
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from http.cookiejar import CookieJar
from typing import Any


class ApiFlowError(RuntimeError):
    pass


class ApiClient:
    def __init__(self, base_url: str, timeout: float) -> None:
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self.cookie_jar = CookieJar()
        self.opener = urllib.request.build_opener(
            urllib.request.HTTPCookieProcessor(self.cookie_jar)
        )

    def request(
        self,
        method: str,
        path: str,
        *,
        body: dict[str, Any] | None = None,
        headers: dict[str, str] | None = None,
    ) -> tuple[int, Any]:
        url = path if path.startswith("http") else f"{self.base_url}{path}"
        data = None
        request_headers = {
            "Accept": "application/json",
            "User-Agent": "afb-new-api-e2e/1.0",
        }
        if body is not None:
            data = json.dumps(body).encode("utf-8")
            request_headers["Content-Type"] = "application/json"
        if headers:
            request_headers.update(headers)

        req = urllib.request.Request(
            url,
            data=data,
            headers=request_headers,
            method=method.upper(),
        )
        try:
            with self.opener.open(req, timeout=self.timeout) as resp:
                raw = resp.read()
                return resp.status, self._decode(raw)
        except urllib.error.HTTPError as exc:
            raw = exc.read()
            return exc.code, self._decode(raw)
        except urllib.error.URLError as exc:
            raise ApiFlowError(f"Cannot connect to {url}: {exc}") from exc

    @staticmethod
    def _decode(raw: bytes) -> Any:
        text = raw.decode("utf-8", errors="replace")
        if not text:
            return None
        try:
            return json.loads(text)
        except json.JSONDecodeError:
            return text


def random_suffix(length: int = 8) -> str:
    alphabet = string.ascii_lowercase + string.digits
    return "".join(random.choice(alphabet) for _ in range(length))


def expect_api_success(label: str, status: int, payload: Any) -> dict[str, Any]:
    if not isinstance(payload, dict):
        raise ApiFlowError(f"{label} returned non-JSON response [{status}]: {payload!r}")
    if status >= 400 or payload.get("success") is False:
        message = payload.get("message") or payload.get("error") or payload
        raise ApiFlowError(f"{label} failed [{status}]: {message}")
    return payload


def mask_secret(value: str, keep_start: int = 6, keep_end: int = 4) -> str:
    if len(value) <= keep_start + keep_end:
        return "*" * len(value)
    return f"{value[:keep_start]}...{value[-keep_end:]}"


def extract_items(payload: dict[str, Any]) -> list[dict[str, Any]]:
    data = payload.get("data")
    if isinstance(data, dict):
        items = data.get("items")
        if isinstance(items, list):
            return [item for item in items if isinstance(item, dict)]
    return []


def choose_model(models_payload: dict[str, Any], override: str | None) -> str:
    if override:
        return override
    models = models_payload.get("data")
    if not isinstance(models, list) or not models:
        raise ApiFlowError("GET /api/user/models returned no usable models.")
    first = models[0]
    if isinstance(first, str):
        return first
    if isinstance(first, dict) and isinstance(first.get("id"), str):
        return first["id"]
    raise ApiFlowError(f"Cannot parse first model from response: {first!r}")


def print_step(message: str) -> None:
    print(f"\n==> {message}")


def run_flow(args: argparse.Namespace) -> None:
    client = ApiClient(args.base_url, args.timeout)
    now = time.strftime("%Y%m%d_%H%M%S")
    # Backend validates username with max=20, so keep generated names compact.
    username = args.username or f"e2e{random_suffix(10)}"
    password = args.password or f"E2e_{random_suffix(10)}_Pass9"
    email = args.email or f"{username}@example.local"

    print(f"Base URL: {client.base_url}")
    print(f"Username: {username}")
    print(f"Password: {password}")
    print(f"Email: {email}")

    if not args.skip_register:
        print_step("Register user")
        register_status, register_payload = client.request(
            "POST",
            "/api/user/register",
            body={
                "username": username,
                "password": password,
                "email": email,
            },
        )
        expect_api_success("POST /api/user/register", register_status, register_payload)
        print("Registered successfully.")

    print_step("Login and get user access token")
    login_status, login_payload = client.request(
        "POST",
        "/api/user/login",
        body={"username": username, "password": password},
    )
    login_data = expect_api_success("POST /api/user/login", login_status, login_payload).get(
        "data", {}
    )
    if login_data.get("require_2fa"):
        raise ApiFlowError("Login requires 2FA; this E2E script cannot continue automatically.")
    user_id = login_data.get("id")
    access_token = login_data.get("access_token")
    if not isinstance(user_id, int) or not isinstance(access_token, str) or not access_token:
        raise ApiFlowError(f"Login response missed id/access_token: {login_data}")
    print(f"Logged in as user #{user_id}; access_token={mask_secret(access_token)}")

    user_headers = {
        "New-Api-User": str(user_id),
        # User management endpoints expect the raw access_token, not a Bearer value.
        "Authorization": access_token,
    }

    print_step("Query user balance/profile")
    self_status, self_payload = client.request(
        "GET",
        "/api/user/self",
        headers=user_headers,
    )
    self_data = expect_api_success("GET /api/user/self", self_status, self_payload).get(
        "data", {}
    )
    print(
        "User quota:",
        json.dumps(
            {
                "quota": self_data.get("quota"),
                "used_quota": self_data.get("used_quota"),
                "request_count": self_data.get("request_count"),
                "group": self_data.get("group"),
            },
            ensure_ascii=False,
        ),
    )

    print_step("Create unlimited default-group API token")
    token_name = f"e2e-token-{now}-{random_suffix(4)}"
    create_status, create_payload = client.request(
        "POST",
        "/api/token/",
        headers=user_headers,
        body={
            "name": token_name,
            "expired_time": -1,
            "remain_quota": 0,
            "unlimited_quota": True,
            "model_limits_enabled": False,
            "model_limits": "",
            "allow_ips": "",
            "group": "",
            "cross_group_retry": False,
        },
    )
    expect_api_success("POST /api/token/", create_status, create_payload)
    print(f"Created token: {token_name}")

    print_step("Find token id from token list")
    query = urllib.parse.urlencode({"p": 1, "page_size": 100, "size": 100})
    list_status, list_payload = client.request(
        "GET",
        f"/api/token/?{query}",
        headers=user_headers,
    )
    list_payload = expect_api_success("GET /api/token/", list_status, list_payload)
    token_items = extract_items(list_payload)
    token = next((item for item in token_items if item.get("name") == token_name), None)
    if token is None:
        raise ApiFlowError(f"Created token {token_name!r} was not found in token list.")
    token_id = token.get("id")
    if not isinstance(token_id, int):
        raise ApiFlowError(f"Created token has invalid id: {token!r}")
    print(f"Token id: {token_id}; masked list key={token.get('key')}")

    print_step("Fetch full token key")
    key_status, key_payload = client.request(
        "POST",
        f"/api/token/{token_id}/key",
        headers=user_headers,
    )
    key_data = expect_api_success(
        f"POST /api/token/{token_id}/key", key_status, key_payload
    ).get("data", {})
    full_key = key_data.get("key")
    if not isinstance(full_key, str) or not full_key:
        raise ApiFlowError(f"Token key response missed data.key: {key_payload}")
    api_key = full_key if full_key.startswith("sk-") else f"sk-{full_key}"
    print(f"Full API key: {mask_secret(api_key)}")

    print_step("Query user-available models")
    models_status, models_payload = client.request(
        "GET",
        "/api/user/models",
        headers=user_headers,
    )
    models_payload = expect_api_success(
        "GET /api/user/models", models_status, models_payload
    )
    model = choose_model(models_payload, args.model)
    print(f"Selected model: {model}")

    if args.skip_chat:
        print("\nSkipped chat completion because --skip-chat was set.")
        return

    print_step("Call OpenAI-compatible /v1/chat/completions")
    chat_status, chat_payload = client.request(
        "POST",
        "/v1/chat/completions",
        headers={"Authorization": f"Bearer {api_key}"},
        body={
            "model": model,
            "messages": [
                {
                    "role": "user",
                    "content": args.prompt,
                }
            ],
            "stream": False,
        },
    )
    if chat_status >= 400:
        raise ApiFlowError(
            f"POST /v1/chat/completions failed [{chat_status}]: {chat_payload}"
        )
    if not isinstance(chat_payload, dict):
        raise ApiFlowError(f"Chat returned non-JSON response: {chat_payload!r}")

    choices = chat_payload.get("choices")
    content = None
    if isinstance(choices, list) and choices:
        first_choice = choices[0]
        if isinstance(first_choice, dict):
            message = first_choice.get("message")
            if isinstance(message, dict):
                content = message.get("content")
            content = content or first_choice.get("text")
    print("Chat response:")
    print(content if content else json.dumps(chat_payload, ensure_ascii=False, indent=2))


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Run local New API user/token/model/chat E2E flow."
    )
    parser.add_argument(
        "--base-url",
        default=os.getenv("BASE_URL", "http://localhost:3000"),
        help="Backend base URL. Default: %(default)s",
    )
    parser.add_argument(
        "--username",
        default=os.getenv("E2E_USERNAME"),
        help="Use an existing/specific username instead of generating one.",
    )
    parser.add_argument(
        "--password",
        default=os.getenv("E2E_PASSWORD"),
        help="Password for the generated or specified user.",
    )
    parser.add_argument(
        "--email",
        default=os.getenv("E2E_EMAIL"),
        help="Email for registration. Default: generated example.local address.",
    )
    parser.add_argument(
        "--model",
        default=os.getenv("E2E_MODEL"),
        help="Override model instead of using the first /api/user/models item.",
    )
    parser.add_argument(
        "--prompt",
        default=os.getenv("E2E_PROMPT", "请用一句话回复：本地接口全流程测试成功。"),
        help="Prompt sent to /v1/chat/completions.",
    )
    parser.add_argument(
        "--timeout",
        type=float,
        default=float(os.getenv("E2E_TIMEOUT", "60")),
        help="HTTP timeout in seconds. Default: %(default)s",
    )
    parser.add_argument(
        "--skip-register",
        action="store_true",
        help="Skip registration and login with --username/--password.",
    )
    parser.add_argument(
        "--skip-chat",
        action="store_true",
        help="Stop after getting the API key and first available model.",
    )
    return parser.parse_args()


def main() -> int:
    try:
        run_flow(parse_args())
    except KeyboardInterrupt:
        print("\nInterrupted.", file=sys.stderr)
        return 130
    except ApiFlowError as exc:
        print(f"\nE2E flow failed: {exc}", file=sys.stderr)
        return 1
    except Exception as exc:  # noqa: BLE001 - this is a CLI boundary.
        print(f"\nUnexpected error: {exc}", file=sys.stderr)
        return 1
    print("\nE2E flow completed.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
