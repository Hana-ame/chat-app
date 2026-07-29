import os
import subprocess
import sys
import time

CWD = os.getcwd()
BINARY = "chatd.exe"
BINARY_PATH = os.path.join(CWD, BINARY)
LOG_FILE = os.path.join(CWD, "server.log")
ENV_FILE = os.path.join(CWD, ".env")
CLIENT_DIR = os.path.join(CWD, "client")


def run(cmd, cwd=None, desc=""):
    print(f"[deploy] {desc}..." if desc else f"[deploy] $ {cmd}")
    subprocess.run(cmd, shell=True, cwd=cwd or CWD, check=True)


def kill_existing():
    print("[deploy] killing existing server...")
    subprocess.run(["taskkill", "/f", "/im", BINARY], capture_output=True)
    time.sleep(1)


def build():
    run("npm ci && npm run build", cwd=CLIENT_DIR, desc="building frontend")
    run("go build -ldflags=\"-s -w -X main.Version=dev\" -o " + BINARY_PATH + " ./cmd/chatd/",
        cwd=os.path.join(CWD, "server"), desc="building backend")


def start():
    print("[deploy] starting server...")
    env = os.environ.copy()
    if os.path.isfile(ENV_FILE):
        with open(ENV_FILE) as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#") or "=" not in line:
                    continue
                k, _, v = line.partition("=")
                env[k.strip()] = v.strip()
    env["CHAT_STATIC_DIR"] = os.path.join(CWD, "client", "dist")
    env["LOG_LEVEL"] = "DEBUG"

    with open(LOG_FILE, "w") as log:
        proc = subprocess.Popen(
            [BINARY_PATH],
            cwd=CWD,
            env=env,
            stdout=log,
            stderr=subprocess.STDOUT,
        )
    print(f"[deploy] started PID {proc.pid}, log: {LOG_FILE}")

    time.sleep(2)
    try:
        import urllib.request
        r = urllib.request.urlopen("http://localhost:8080/api/version", timeout=5)
        print(f"[deploy] OK: {r.read().decode().strip()}")
    except Exception as e:
        print(f"[deploy] WARN: health check failed: {e}")


def main():
    cmd = sys.argv[1] if len(sys.argv) > 1 else "all"
    if cmd == "kill":
        kill_existing()
    elif cmd == "build":
        build()
    elif cmd == "start":
        start()
    elif cmd == "all":
        kill_existing()
        build()
        start()
    elif cmd == "restart":
        kill_existing()
        start()
    else:
        print(f"usage: python {sys.argv[0]} [all|build|start|kill|restart]")
        sys.exit(1)


if __name__ == "__main__":
    main()
