import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tarfile
import urllib.request
from urllib.request import ProxyHandler, build_opener, install_opener

REPO = "Hana-ame/chat-app"
REPO_BASE = f"https://raw.githubusercontent.com/{REPO}/main"
SCRIPT_REPO_PATH = "scripts/deploy_win.py"
BINARY = "chatd-windows-amd64.exe"
CLIENT_ARCHIVE = "client-dist.tar.gz"
CWD = os.getcwd()
SCRIPT_PATH = os.path.abspath(__file__)


def setup_proxy(proxy):
    if not proxy:
        return
    os.environ.setdefault("HTTPS_PROXY", proxy)
    os.environ.setdefault("HTTP_PROXY", proxy)
    handler = ProxyHandler({"http": proxy, "https": proxy})
    install_opener(build_opener(handler))


def fetch_raw(path):
    url = f"{REPO_BASE}/{path}"
    try:
        req = urllib.request.Request(url, headers={"User-Agent": "deploy_win.py"})
        with urllib.request.urlopen(req, timeout=15) as r:
            return r.read().decode("utf-8")
    except Exception as e:
        print(f"[deploy]  WARN: fetch {path} failed: {e}")
        return None


def self_update():
    remote = fetch_raw(SCRIPT_REPO_PATH)
    if remote is None:
        print("[deploy]  SKIP: script self-update check failed")
        return
    with open(SCRIPT_PATH, encoding="utf-8") as f:
        local = f.read()
    if local == remote:
        print("[deploy]  OK: script is up to date")
        return
    print("[deploy]  UPDATE: new version found, applying...")
    with open(SCRIPT_PATH, "w", encoding="utf-8") as f:
        f.write(remote)
    print("[deploy]  UPDATE: script updated, restarting...")
    subprocess.run([sys.executable] + sys.argv)
    sys.exit(0)


def sync_env():
    example_raw = fetch_raw(".env.example")
    if example_raw is None:
        print("[deploy]  SKIP: cannot fetch .env.example from repo")
        return False

    example_path = os.path.join(CWD, ".env.example")
    env_path = os.path.join(CWD, ".env")

    with open(example_path, "w", encoding="utf-8") as f:
        f.write(example_raw)
    print("[deploy]  OK: .env.example synced from repo")

    example_keys = _parse_env_file(example_path)
    if example_keys is None:
        print("[deploy]  WARN: cannot parse .env.example")
        return False

    env_keys = _parse_env_file(env_path)
    if env_keys is None:
        print("[deploy]  INFO: creating .env from latest template")
        shutil.copy2(example_path, env_path)
        print("[deploy]  WARN: edit .env to set secrets before running")
        return False

    modified = False
    with open(env_path, encoding="utf-8") as f:
        env_lines = f.readlines()

    existing_keys = {}
    for i, line in enumerate(env_lines):
        m = re.match(r"^(\w+)=", line.strip())
        if m:
            existing_keys[m.group(1)] = i

    for k, v in example_keys.items():
        if k in ("CHAT_STATIC_DIR",):
            continue
        if k not in existing_keys:
            env_lines.append(f"{k}={v}\n")
            print(f"[deploy]  ADD: {k}={v}")
            modified = True
        else:
            idx = existing_keys[k]
            line = env_lines[idx].strip()
            if not line or line.startswith("#"):
                continue
            _, _, existing_val = line.partition("=")
            if existing_val.strip().strip('"').strip("'") == v and (
                "change-me" in v.lower() or "sk-" in v.lower() or v.strip() == ""
            ):
                print(f"[deploy]  WARN: {k} is still a placeholder in .env")

    if modified:
        with open(env_path, "w", encoding="utf-8") as f:
            f.writelines(env_lines)
        print("[deploy]  OK: .env updated with missing keys")
    else:
        print("[deploy]  OK: .env is up to date")
    return True


def latest_release():
    url = f"https://api.github.com/repos/{REPO}/releases"
    print(f"[deploy] fetching latest release from: {url}")
    req = urllib.request.Request(url, headers={"User-Agent": "deploy_win.py"})
    with urllib.request.urlopen(req) as r:
        releases = json.loads(r.read())
    if not releases:
        print("[deploy] ERROR: no releases found")
        sys.exit(1)
    tag = releases[0].get("tag_name", "?")
    print(f"[deploy] latest release tag: {tag}")
    return releases[0]


def find_asset(release, name):
    for a in release.get("assets", []):
        if a["name"] == name:
            print(f"[deploy] found asset: {a['name']} ({a['size']} bytes)")
            return a
    print(f"[deploy] ERROR: asset {name} not found in release {release['tag_name']}")
    print(f"[deploy] available assets: {[a['name'] for a in release.get('assets', [])]}")
    sys.exit(1)


def download(asset, dst, proxy):
    url = asset["browser_download_url"]
    print(f"[deploy] download url: {url}")
    if os.path.isfile(dst):
        print(f"[deploy] removing existing file: {dst}")
        os.remove(dst)
    cmd = ["curl.exe", "-L", "--progress-bar", "-o", dst]
    if proxy:
        cmd += ["-x", proxy]
    cmd += [url]
    print(f"[deploy] running: {' '.join(cmd)}")
    subprocess.run(cmd, check=True)
    size = os.path.getsize(dst)
    print(f"[deploy] saved: {dst} ({size} bytes)")


def _parse_env_file(path):
    keys = {}
    try:
        with open(path, encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#"):
                    continue
                m = re.match(r"^(\w+)=(.+)?", line)
                if m:
                    keys[m.group(1)] = m.group(2) or ""
    except FileNotFoundError:
        return None
    return keys


def load_env(env_file):
    env = os.environ.copy()
    if env_file and os.path.isfile(env_file):
        print(f"[deploy] loading env from: {env_file}")
        with open(env_file, encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#") or "=" not in line:
                    continue
                k, _, v = line.partition("=")
                env[k.strip()] = v.strip()
        print(f"[deploy] env keys loaded: {[k for k in env if k.startswith('CHAT_')]}")
    else:
        print(f"[deploy] no .env file at {env_file}, using system env only")
    return env


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("cmd", nargs="?", default="all", choices=["download", "run", "all"])
    parser.add_argument("--proxy", default="http://localhost:10809", help="proxy URL")
    parser.add_argument("--no-self-update", action="store_true", help="skip script self-update")
    args = parser.parse_args()

    cmd = args.cmd
    proxy = args.proxy
    print(f"[deploy] command: {cmd}")
    print(f"[deploy] proxy: {proxy if proxy else '(none)'}")
    print(f"[deploy] working dir: {CWD}")
    print(f"[deploy] binary: {BINARY}")
    setup_proxy(proxy)

    if cmd != "run" and not args.no_self_update:
        self_update()

    sync_env()

    dst = os.path.join(CWD, BINARY)
    env_file = os.path.join(CWD, ".env")

    if cmd in ("download", "all"):
        rel = latest_release()
        binary_asset = find_asset(rel, BINARY)
        download(binary_asset, dst, proxy)
        client_asset = find_asset(rel, CLIENT_ARCHIVE)
        client_dst = os.path.join(CWD, CLIENT_ARCHIVE)
        download(client_asset, client_dst, proxy)
        client_dir = os.path.join(CWD, "client", "dist")
        os.makedirs(client_dir, exist_ok=True)
        print(f"[deploy] extracting {CLIENT_ARCHIVE} -> {client_dir}")
        with tarfile.open(client_dst) as tf:
            tf.extractall(client_dir)
        os.remove(client_dst)
        abs_static = client_dir.replace("/", "\\")
        static_line = f"CHAT_STATIC_DIR={abs_static}"
        if os.path.isfile(env_file):
            with open(env_file, encoding="utf-8") as f:
                lines = f.readlines()
            with open(env_file, "w", encoding="utf-8") as f:
                found = False
                for line in lines:
                    if line.strip().startswith("CHAT_STATIC_DIR="):
                        f.write(static_line + "\n")
                        found = True
                    else:
                        f.write(line)
                if not found:
                    f.write(static_line + "\n")
        else:
            with open(env_file, "a", encoding="utf-8") as f:
                f.write(static_line + "\n")
        print(f"[deploy] download complete (tag: {rel['tag_name']})")

    if cmd in ("run", "all"):
        if not os.path.isfile(dst):
            print(f"[deploy] ERROR: {dst} not found, run download first")
            sys.exit(1)
        print(f"[deploy] binary exists: {dst} ({os.path.getsize(dst)} bytes)")
        if not os.path.isfile(env_file):
            example = os.path.join(CWD, ".env.example")
            if os.path.isfile(example):
                shutil.copy2(example, env_file)
                print(f"[deploy] created {env_file} from .env.example")
            else:
                print(f"[deploy] no .env.example at {example}, skipping")
        env = load_env(env_file)
        os.chdir(CWD)
        print(f"[deploy] starting: {dst}")
        print(f"[deploy] --- server output below ---")
        subprocess.run([dst], env=env)

    if cmd not in ("download", "run", "all"):
        print(f"usage: python {sys.argv[0]} [download|run|all]")
        sys.exit(1)


if __name__ == "__main__":
    main()
