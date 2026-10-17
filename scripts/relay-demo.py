#!/usr/bin/env python3
"""Isolated process smoke test; owns and removes every process and configuration."""
import contextlib
import http.client
import http.server
import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import tempfile
import threading
import time

ROOT = Path(__file__).resolve().parents[1]


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def request(port, path, method="GET", body=None, host=None, csrf=None):
    conn = http.client.HTTPConnection("127.0.0.1", port, timeout=5)
    headers = {"Host": host or f"localhost:{port}"}
    if body is not None:
        headers.update({"Origin": f"http://localhost:{port}", "X-CSRF-Token": csrf,
                        "Content-Type": "application/json"})
    conn.request(method, path, json.dumps(body) if body is not None else None, headers)
    response = conn.getresponse()
    data = response.read()
    status = response.status
    conn.close()
    return status, data


def state(port):
    status, body = request(port, "/api/state")
    assert status == 200, (status, body)
    return json.loads(body)


def mutate(port, path, method, body):
    for _ in range(30):
        current = state(port)
        body["revision"] = current["revision"]
        status, data = request(port, path, method, body, csrf=current["csrf"])
        if status == 409:
            continue
        assert status in (200, 201), (status, data)
        return json.loads(data)
    raise AssertionError("configuration remained conflicted")


def wait_for(fn):
    deadline = time.monotonic() + 15
    while time.monotonic() < deadline:
        try:
            value = fn()
            if value:
                return value
        except (ConnectionError, OSError, AssertionError):
            pass
        time.sleep(.05)
    raise AssertionError("condition did not become ready")


class Backend(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(403)
        self.end_headers()
        self.wfile.write((self.headers["Host"] + " " + self.path).encode())

    def log_message(self, *_):
        pass


def main():
    processes = []
    with tempfile.TemporaryDirectory(prefix="zelport-relay-demo-") as directory:
        work = Path(directory)
        binary = work / "zelport"
        subprocess.run(["go", "build", "-tags", "local_ports,ports_over_relay", "-o", str(binary), "./cmd/zelport"], cwd=ROOT, check=True)
        relay = work / "zelport-relay"
        subprocess.run(["go", "build", "-o", str(relay), "./cmd/zelport-relay"], cwd=ROOT.parent / "zelport-relay", check=True)
        subprocess.run([str(relay), "token", "--out", str(work / "access")], check=True)
        access = (work / "access").read_text().strip()
        group = secrets.token_urlsafe(32)
        relay_port, a, b = free_port(), free_port(), free_port()
        backend = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Backend)
        threading.Thread(target=backend.serve_forever, daemon=True).start()
        logs = []

        def start(args):
            log = (work / f"process-{len(logs)}.log").open("w+")
            logs.append(log)
            proc = subprocess.Popen(args, cwd=ROOT, stdout=log, stderr=log)
            processes.append(proc)
            return proc

        try:
            relay_args = [str(relay), "--listen", f"127.0.0.1:{relay_port}", "--access-token-file", str(work / "access")]
            relay_process = start(relay_args)
            start([str(binary), "--port", str(a), "--config", str(work / "a.json")])
            start([str(binary), "--port", str(b), "--config", str(work / "b.json")])
            wait_for(lambda: state(a)); wait_for(lambda: state(b))
            source = mutate(a, "/api/routes", "POST", {"route": {"module": "local_ports", "domain": "demo.localhost", "settings": {"aux_port": backend.server_port}}})["route"]
            settings = {"enabled": True, "publish": True, "relay_url": f"ws://127.0.0.1:{relay_port}", "access_token": access, "group_token": group}
            mutate(a, "/api/modules/ports_over_relay/settings", "PUT", settings.copy())
            settings["publish"] = False
            mutate(b, "/api/modules/ports_over_relay/settings", "PUT", settings.copy())

            def offer():
                data = state(b)["states"]["ports_over_relay"]["data"]["status"]
                if not data["connected"]:
                    return None
                return next((r for r in data["records"] if any(s["id"] == source["id"] for s in r["services"])), None)

            record = wait_for(offer)
            for domain in ["demo.localhost", "renamed.localhost"]:
                mutate(b, "/api/routes", "POST", {"route": {"module": "ports_over_relay", "domain": domain, "settings": {"group": record["group"], "device_id": record["device_id"], "service_id": source["id"], "provider_key": record["public_key"], "rewrite_origin": False}}})
                wait_for(lambda: request(b, "/path?q=1", host=f"{domain}:{b}") == (403, f"demo.localhost:{a} /path?q=1".encode()))
            settings["publish"] = False
            mutate(a, "/api/modules/ports_over_relay/settings", "PUT", settings.copy())
            wait_for(lambda: request(b, "/", host=f"demo.localhost:{b}")[0] == 503)
            settings["publish"] = True
            mutate(a, "/api/modules/ports_over_relay/settings", "PUT", settings.copy())
            wait_for(lambda: request(b, "/", host=f"demo.localhost:{b}")[0] == 403)
            relay_process.terminate(); relay_process.wait(timeout=10)
            wait_for(lambda: request(b, "/", host=f"demo.localhost:{b}")[0] == 503)
            start(relay_args)
            wait_for(lambda: request(b, "/", host=f"demo.localhost:{b}")[0] == 403)
            assert len(state(b)["states"]["ports_over_relay"]["routes"]) == 2
            print("PASS: two processes, original/renamed imports, application 403, revoke/re-enable, relay restart, retained imports")
        finally:
            for proc in reversed(processes):
                if proc.poll() is None:
                    proc.terminate()
                try:
                    proc.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    proc.kill(); proc.wait()
            backend.shutdown(); backend.server_close()
            for log in logs:
                log.close()


if __name__ == "__main__":
    main()
