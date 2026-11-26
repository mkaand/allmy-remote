"""CI-only acceptance for the isolated Compose preview. No real credentials."""
import json
import os
import secrets
import subprocess
import time
import urllib.error
import urllib.request

if os.environ.get("CI") != "true":
    raise SystemExit("This destructive fixture is restricted to an isolated CI runner.")

PROJECT = "allmy-remote-ci-persistence"
PORT = "18081"
ORIGIN = "http://localhost:" + PORT
ENV = dict(os.environ, ALLMY_PREVIEW_PORT=PORT)
COMPOSE = ["docker", "compose", "-p", PROJECT, "-f", "compose.preview.yml"]


def run(*args, input_text=None):
    result = subprocess.run(
        [*COMPOSE, *args], env=ENV, input=input_text, text=True,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=60,
    )
    if result.returncode:
        # Do not print stdout or stdin; initializer secrets must never reach CI logs.
        detail = result.stderr
        if input_text:
            detail = detail.replace(input_text.strip(), "[REDACTED]")
        raise RuntimeError(f"Compose {args[0]} failed ({result.returncode}): {detail.strip()}")
    return result.stdout.strip()


def request(path, method="GET", body=None, cookie=None, origin=None, want=200):
    headers = {}
    if body is not None:
        headers["Content-Type"] = "application/json"
    if cookie:
        headers["Cookie"] = cookie
    if origin:
        headers["Origin"] = origin
    req = urllib.request.Request(
        ORIGIN + path, method=method, headers=headers,
        data=json.dumps(body).encode() if body is not None else None,
    )
    try:
        response = urllib.request.urlopen(req, timeout=5)
    except urllib.error.HTTPError as exc:
        response = exc
    with response:
        assert response.status == want, f"{method} {path}: {response.status}, expected {want}"
        return json.load(response), response.headers


def ready():
    for _ in range(50):
        try:
            result, _ = request("/ready")
            assert result["status"] == "ready"
            return
        except (OSError, AssertionError):
            time.sleep(0.2)
    raise AssertionError("Preview did not become ready")


def login(password):
    _, headers = request("/api/v1/session/login", "POST",
                         {"username": "admin", "password": password}, origin=ORIGIN)
    cookie = headers["Set-Cookie"]
    for flag in ("Secure", "HttpOnly", "SameSite=Strict"):
        assert flag in cookie
    return cookie.split(";", 1)[0]


def inspect_container():
    cid = run("ps", "-q", "server")
    assert cid
    obj = json.loads(subprocess.check_output(["docker", "inspect", cid], text=True))[0]
    host = obj["HostConfig"]
    bindings = host["PortBindings"]["8080/tcp"]
    assert len(bindings) == 1 and bindings[0]["HostIp"] == "127.0.0.1"
    assert host["ReadonlyRootfs"]
    assert "ALL" in host["CapDrop"]
    assert obj["Config"]["User"] == "65532:65532"
    assert any(x["Type"] == "volume" and x["Destination"] == "/data" for x in obj["Mounts"])
    return cid


def main():
    password = secrets.token_urlsafe(24)
    run("run", "--rm", "-T", "--interactive", "--no-deps", "server", "init-admin", input_text=password + "\n")
    print("PASS: non-root administrator bootstrap in persistent volume", flush=True)
    run("up", "-d", "--no-build")
    ready()
    first = inspect_container()
    request("/api/v1/devices", want=401)
    cookie = login(password)
    grant, _ = request("/api/v1/enrollment/grants", "POST", {"name": "CI persistence mock"},
                       cookie=cookie, origin=ORIGIN, want=201)
    device, _ = request("/api/v1/enrollment/exchange", "POST", {"grant": grant["grant"]}, want=201)
    request("/api/v1/enrollment/exchange", "POST", {"grant": grant["grant"]}, want=401)
    device_id = device["id"]
    request("/api/v1/devices/" + device_id + "/revoke", "POST", cookie=cookie, origin=ORIGIN)

    run("stop", "server")
    stopped = run("ps", "--status", "running", "-q", "server")
    assert stopped == "", "Server did not stop"
    run("rm", "-f", "server")
    run("up", "-d", "--no-build")
    ready()
    assert inspect_container() != first, "Container was not recreated"
    cookie = login(password)
    devices, _ = request("/api/v1/devices", cookie=cookie)
    assert len(devices) == 1
    assert devices[0]["id"] == device_id and devices[0]["status"] == "revoked"
    assert devices[0]["platform"] == "mock"
    assert "credential" not in devices[0] and "credential_hash" not in devices[0]
    request("/api/v1/session/logout", "POST", cookie=cookie, origin=ORIGIN)
    request("/api/v1/devices", cookie=cookie, want=401)
    print("PASS: loopback, non-root, read-only rootfs, persistent admin and revoked device, logout")


try:
    main()
finally:
    # Only this uniquely named CI fixture; never invoke on a real deployment.
    run("down", "-v", "--remove-orphans")
