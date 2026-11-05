#!/usr/bin/env python3
"""TON Meme Sniper portable route. NO ADNL, NO UDP, HTTPS only.

Single portable pipeline, Python stdlib only:
  webhook receiver -> fast pre-filter -> enrich -> alert-ready output.

How it runs on any host (Linux, server, Termux phone):
  WEBHOOK_TOKEN=<your-webhook-token> TONAPI_KEY=<your-tonapi-key> \
    python3 pipeline.py --port 8765
  TONAPI_KEY is optional: without it TonAPI answers unauthenticated
  (rate limited, about 1 request per 4 seconds). WEBHOOK_TOKEN is
  optional for local tests only; in production it must be set.
  No key, token, host, or DEX address is hard-coded anywhere here.

Pipeline, per event, in order:
  1. Receiver: POST /webhook -> 200 on valid JSON, 400 on invalid
     JSON, 401 on a wrong Bearer token, duplicate Idempotency-Key
     acknowledged with 200 but not logged or processed twice.
  2. Pre-filter: ONE TonAPI account call only.
       - missing account_id -> DROP, 0 calls
       - is_wallet true, or interfaces contain a wallet_* entry -> DROP wallet, 1 call
         (jetton_wallet is checked separately, after this, so it is not
         mistaken for a wallet)
       - interfaces contain jetton_wallet -> DROP jetton_wallet, 1 call
       - interfaces contain a pool marker (stonfi_pool_*, dedust_pool,
         or any interface containing "pool") -> DROP dex_pool, 1 call
       - interfaces do NOT contain jetton_master -> DROP not_a_jetton, 1 call
       - otherwise (jetton_master, no pool marker, not a wallet) -> PASS candidate
  3. Enrich, PASS candidates only: TonAPI jetton + jetton-holders
     (2 more calls, 3 total). Output name, symbol, decimals,
     total_supply, mintable, admin, holders_count, verification,
     top holder and top holder share (balance / total_supply, both
     numbers shown). Missing fields stay "not available" / "unknown".
     No score, no Gem wording, no safe / not-a-rug label.
  4. Output: PASS lines go to alerts.jsonl, DROP lines to
     dropped.jsonl, every accepted event to events.jsonl, all next
     to this script unless --out-dir is given.

Phase 1 only: detection + alert-ready log. No wallet, no signing,
no trade, no auto-buy, no Vault, no webhook creation here. A live
webhook subscription is a separate, short, cleaned-up test only.

TonAPI calls in this file are plain HTTPS GETs to tonapi.io/v2.
The caller supplies TONAPI_KEY in the environment on their own host;
this file never reads a Muse connector, never reads a file of secrets,
and never logs an Authorization header value.
"""
import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

POOL_MARKERS = ("stonfi_pool", "dedust_pool", "pool")
SEEN_KEYS = set()


def utcnow():
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def tonapi_get(path, timeout=30):
    """GET https://tonapi.io/v2<path>. Return (http_status, json_or_None, ms).

    TONAPI_KEY is read from the environment at call time, never logged.
    """
    url = "https://tonapi.io/v2" + path
    headers = {}
    key = os.environ.get("TONAPI_KEY")
    if key:
        headers["Authorization"] = "Bearer " + key
    req = urllib.request.Request(url, headers=headers)
    t0 = time.monotonic()
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            ms = round((time.monotonic() - t0) * 1000, 1)
            return resp.status, json.loads(resp.read().decode("utf-8", "replace")), ms
    except urllib.error.HTTPError as e:
        ms = round((time.monotonic() - t0) * 1000, 1)
        return e.code, None, ms
    except Exception:
        ms = round((time.monotonic() - t0) * 1000, 1)
        return 0, None, ms


def process_event(event, fetch=tonapi_get):
    """Run pre-filter + enrich for one event dict. Return a result dict.

    `fetch` is injectable for unit tests: it takes a TonAPI path and
    returns (http_status, json_or_None, ms). Production uses tonapi_get.
    """
    t_start = time.monotonic()
    received_at = event.get("received_at") or event.get("received_at_utc") or utcnow()
    account_id = event.get("account_id")
    base = {
        "received_at": received_at,
        "account_id": account_id if account_id else "unknown",
        "lt": event.get("lt", "not available"),
        "tx_hash": event.get("tx_hash", "not available"),
    }
    if not account_id:
        base.update({"decision": "DROP", "reason": "missing_account_id",
                     "calls": [], "call_count": 0,
                     "pipeline_ms_total": round((time.monotonic() - t_start) * 1000, 1)})
        return base

    calls = []
    status_a, account, ms_a = fetch("/accounts/" + urllib.parse.quote(str(account_id)))
    calls.append({"endpoint": "account", "http_status": status_a, "ms": ms_a})
    interfaces = (account or {}).get("interfaces") or []
    is_wallet = bool((account or {}).get("is_wallet"))
    if status_a != 200:
        base.update({"decision": "DROP", "reason": "account_lookup_failed",
                     "account_interfaces": interfaces if interfaces else ["unknown"],
                     "is_wallet": is_wallet, "calls": calls, "call_count": len(calls),
                     "pipeline_ms_total": round((time.monotonic() - t_start) * 1000, 1)})
        return base

    def drop(reason):
        base.update({"decision": "DROP", "reason": reason,
                     "account_interfaces": interfaces if interfaces else ["unknown"],
                     "is_wallet": is_wallet, "calls": calls, "call_count": len(calls),
                     "pipeline_ms_total": round((time.monotonic() - t_start) * 1000, 1)})
        return base

    if is_wallet or any(i.startswith("wallet_") for i in interfaces):
        return drop("wallet")
    if "jetton_wallet" in interfaces:
        return drop("jetton_wallet")
    if any(any(m in i for i in interfaces) for m in POOL_MARKERS):
        return drop("dex_pool")
    if "jetton_master" not in interfaces:
        return drop("not_a_jetton")

    # Surviving jetton_master candidate only: enrich with 2 more calls.
    status_j, jetton, ms_j = fetch("/jettons/" + urllib.parse.quote(str(account_id)))
    calls.append({"endpoint": "jetton", "http_status": status_j, "ms": ms_j})
    if status_j != 200 or not jetton:
        return drop("jetton_lookup_failed")
    metadata = jetton.get("metadata") or {}
    admin = jetton.get("admin") or {}
    admin_address = admin.get("address", "unknown") if isinstance(admin, dict) else "unknown"
    total_supply = jetton.get("total_supply", "not available")
    status_h, holders, ms_h = fetch("/jettons/" + urllib.parse.quote(str(account_id)) + "/holders")
    calls.append({"endpoint": "jetton-holders", "http_status": status_h, "ms": ms_h})
    top_holder, top_balance, top_share = "not available", "not available", "not available"
    if holders and isinstance(holders.get("addresses"), list) and holders["addresses"]:
        best = None
        for entry in holders["addresses"]:
            try:
                bal = int(entry.get("balance", 0))
            except Exception:
                continue
            if best is None or bal > best[0]:
                best = (bal, (entry.get("owner") or {}).get("address", "unknown"))
        if best is not None:
            top_balance = str(best[0])
            top_holder = best[1]
            try:
                top_share = round(best[0] / int(total_supply) * 100, 2)
            except Exception:
                top_share = "not available"
    base.update({"decision": "PASS", "reason": "jetton_master",
                 "account_interfaces": interfaces,
                 "is_wallet": is_wallet,
                 "name": metadata.get("name", "not available"),
                 "symbol": metadata.get("symbol", "not available"),
                 "decimals": metadata.get("decimals", "not available"),
                 "total_supply": total_supply,
                 "mintable": jetton.get("mintable", "not available"),
                 "admin": admin_address,
                 "holders_count": jetton.get("holders_count", "not available"),
                 "verification": jetton.get("verification", "not available"),
                 "top_holder": top_holder, "top_holder_balance": top_balance,
                 "top_holder_share_percent": top_share,
                 "calls": calls, "call_count": len(calls),
                 "pipeline_ms_total": round((time.monotonic() - t_start) * 1000, 1)})
    return base


def make_handler(out_dir, expected_token):
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            sys.stderr.write("[pipeline] " + " ".join(str(a) for a in args) + "\n")

        def _send(self, code, payload):
            body = json.dumps(payload).encode()
            self.send_response(code)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def _append(self, name, record):
            with open(os.path.join(out_dir, name), "a", encoding="utf-8") as f:
                f.write(json.dumps(record, ensure_ascii=False) + "\n")

        def do_POST(self):
            if self.path != "/webhook":
                self._send(404, {"error": "not found"})
                return
            length = int(self.headers.get("Content-Length") or 0)
            raw = self.rfile.read(length) if length > 0 else b""
            try:
                event = json.loads(raw.decode("utf-8"))
                if not isinstance(event, dict):
                    raise ValueError("event is not a JSON object")
            except Exception:
                self._send(400, {"error": "invalid JSON"})
                return
            if expected_token:
                auth = self.headers.get("Authorization") or ""
                if auth != "Bearer " + expected_token:
                    self._send(401, {"error": "unauthorized"})
                    return
            key = self.headers.get("Idempotency-Key")
            if key and key in SEEN_KEYS:
                self._send(200, {"ok": True, "duplicate": True})
                return
            if key:
                SEEN_KEYS.add(key)
            record = {"received_at_utc": utcnow(), "event": event}
            self._append("events.jsonl", record)
            result = process_event(event)
            self._append("alerts.jsonl" if result["decision"] == "PASS" else "dropped.jsonl", result)
            self._send(200, {"ok": True, "duplicate": False,
                             "decision": result["decision"], "reason": result["reason"]})

        def do_GET(self):
            if self.path == "/health":
                self._send(200, {"ok": True})
                return
            self._send(404, {"error": "not found"})
    return Handler


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--port", type=int, default=8765)
    ap.add_argument("--host", default="127.0.0.1")
    ap.add_argument("--out-dir", default=None)
    ap.add_argument("--events-file", default=None, help="Process a JSONL file instead of serving")
    args = ap.parse_args()
    out_dir = args.out_dir or os.path.dirname(os.path.abspath(__file__))
    os.makedirs(out_dir, exist_ok=True)
    if args.events_file:
        with open(args.events_file, encoding="utf-8") as f:
            lines = [ln for ln in f.read().splitlines() if ln.strip()]
        for line in lines:
            try:
                event = json.loads(line)
            except Exception:
                continue
            result = process_event(event if isinstance(event, dict) else {})
            print(json.dumps(result, ensure_ascii=False))
            with open(os.path.join(out_dir, "alerts.jsonl" if result["decision"] == "PASS" else "dropped.jsonl"), "a", encoding="utf-8") as f:
                f.write(json.dumps(result, ensure_ascii=False) + "\n")
        return 0
    handler = make_handler(out_dir, os.environ.get("WEBHOOK_TOKEN") or None)
    server = ThreadingHTTPServer((args.host, args.port), handler)
    sys.stderr.write(f"[pipeline] listening on {args.host}:{args.port}, out={out_dir}\n")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
