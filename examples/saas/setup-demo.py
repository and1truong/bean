#!/usr/bin/env python3
"""Provision a fresh local SaaS demo using the CLI, Views and Actions."""
import argparse
import http.cookiejar
import json
from pathlib import Path
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parents[2]
PASSWORD = "test-password"
WORKSPACES = [
    ("a", "Northstar Studio", "Client delivery and product launches for the Northstar team.", [
        ("Client onboarding refresh", "Make the first week simpler for new clients.", "planned"),
        ("Design system rollout", "Bring shared components to every client project.", "active"),
        ("Autumn campaign", "Prepare the creative assets and launch checklist.", "active"),
        ("Accessibility review", "Audit navigation and key forms before release.", "planned"),
        ("Customer portal launch", "Ship the first version of the client portal.", "completed"),
        ("Spring microsite", "Keep last season's campaign for reference.", "archived"),
    ]),
    ("b", "Harbor Labs", "Research and internal tools for the Harbor team.", [
        ("Research library", "Organise interviews into a searchable library.", "active"),
        ("Experiment dashboard", "Plan a clear view of product experiments.", "planned"),
        ("Support playbook", "Document the team's support procedures.", "completed"),
    ]),
]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--db", type=Path, default=ROOT / "tmp/saas-workspace.db")
    args = parser.parse_args()
    database = args.db.resolve()
    if database.exists():
        parser.error(f"{database} already exists; serve it or choose a new --db path. No data was changed.")
    database.parent.mkdir(parents=True, exist_ok=True)
    binary = ROOT / "bin/bean"

    def cli(*command):
        subprocess.run([str(binary), *command, "--db", str(database)], check=True, capture_output=True, text=True)

    cli("init", "--admin-email", "admin@example.test", "--admin-password", PASSWORD)
    cli("app", "publish", "--file", str(ROOT / "examples/saas/app.yaml"))
    for suffix, *_ in WORKSPACES:
        for role in ("owner", "member"):
            cli("user", "create", "--email", f"{role}-{suffix}@example.test", "--password", PASSWORD,
                "--roles", role, "--tenant", f"00000000-0000-4000-8000-00000000000{suffix}")

    with socket.socket() as available:
        available.bind(("127.0.0.1", 0))
        port = available.getsockname()[1]
    base = f"http://127.0.0.1:{port}"
    with tempfile.TemporaryFile() as log:
        server = subprocess.Popen([str(binary), "serve", "--db", str(database), "--addr", f"127.0.0.1:{port}"], stdout=log, stderr=log)
        try:
            for _ in range(100):
                try:
                    with urllib.request.urlopen(base + "/healthz", timeout=1):
                        break
                except OSError:
                    if server.poll() is not None:
                        raise RuntimeError("The demo server exited during setup")
                    time.sleep(0.05)
            else:
                raise RuntimeError("The demo server did not become ready")

            for suffix, name, description, projects in WORKSPACES:
                client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
                csrf = ""

                def request(path, data=None):
                    headers = {"Content-Type": "application/json"}
                    if csrf:
                        headers["X-CSRF-Token"] = csrf
                    req = urllib.request.Request(base + path, data=None if data is None else json.dumps(data).encode(), headers=headers)
                    try:
                        with client.open(req, timeout=10) as response:
                            return json.load(response)
                    except urllib.error.HTTPError as error:
                        raise RuntimeError(f"{path}: HTTP {error.code}: {error.read().decode()}") from error

                csrf = request("/api/auth/login", {"email": f"owner-{suffix}@example.test", "password": PASSWORD})["csrfToken"]
                request("/api/actions/organisation_create", {"name": name, "description": description})
                for title, summary, status in projects:
                    record = request("/api/actions/project_create", {"name": title, "description": summary})["data"]
                    steps = {"planned": [], "active": ["start_project"], "completed": ["start_project", "complete_project"], "archived": ["archive_project"]}[status]
                    for action in steps:
                        request("/api/actions/" + action, {"id": record["id"]})
                rows = request("/api/projects")["data"]
                if {(row["name"], row["status"]) for row in rows} != {(title, status) for title, _, status in projects}:
                    raise RuntimeError("Seed verification failed for " + name)
                print(f"{name}: {len(rows)} projects; owner-{suffix}@example.test / member-{suffix}@example.test")
        finally:
            server.terminate()
            server.wait(timeout=10)
    print(f"Ready: {database}\nDemo password: {PASSWORD}")
    print(f"Run: ./bin/bean serve --db {database} --addr 127.0.0.1:8084")


if __name__ == "__main__":
    main()
