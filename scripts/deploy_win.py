# should always push to branch=dev

import argparse
import json
import os
import re
import secrets
import shutil
import subprocess
import sys
import tarfile
import time
from concurrent.futures import ThreadPoolExecutor
import urllib.request
from urllib.request import ProxyHandler, build_opener, install_opener

VERSION = "0.8.1"
REPO = "Hana-ame/chat-app"
REPO_BRANCH = os.environ.get("DEPLOY_BRANCH", "dev")
REPO_BASE = f"https://raw.githubusercontent.com/{REPO}/{REPO_BRANCH}"
BINARY = "chatd-windows-amd64.exe"
CLIENT_ARCHIVE = "client-dist.tar.gz"
CWD = os.getcwd()
KNOWN_TAG_FILE = os.path.join(CWD, ".deployed_tag")

GH_PROXY = "https://gh-proxy.com/"


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
        ensure_jwt_secret(env_path)
        print("[deploy]  OK: .env created with persistent secrets")
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

    ensure_jwt_secret(env_path)
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


def download(asset, dst):
    url = GH_PROXY + asset["browser_download_url"]
    print(f"[deploy] download url: {url}")
    cmd = ["curl.exe", "-L", "-C", "-", "--retry", "5", "--connect-timeout", "30", "--progress-bar", "-o", dst, url]
    print(f"[deploy] running: {' '.join(cmd)}")
    subprocess.run(cmd, check=True)
    size = os.path.getsize(dst)
    print(f"[deploy] saved: {dst} ({size} bytes)")


def kill_chatd():
    subprocess.run(["taskkill", "/f", "/im", BINARY], capture_output=True)
    time.sleep(1)


def start_detached(dst, env):
    startupinfo = subprocess.STARTUPINFO()
    startupinfo.dwFlags |= subprocess.STARTF_USESHOWWINDOW
    proc = subprocess.Popen(
        ["cmd", "/c", "start", "/min", "", dst],
        env=env, startupinfo=startupinfo
    )
    print(f"[deploy] started detached: {dst} (PID: {proc.pid})")
    return proc


def read_known_tag():
    try:
        with open(KNOWN_TAG_FILE) as f:
            return f.read().strip()
    except FileNotFoundError:
        return None


def write_known_tag(tag):
    with open(KNOWN_TAG_FILE, "w") as f:
        f.write(tag)


def ensure_jwt_secret(env_path):
    if not os.path.isfile(env_path):
        return
    lines = []
    found = False
    with open(env_path, encoding="utf-8") as f:
        for line in f:
            m = re.match(r"^CHAT_JWT_SECRET=(.+)?", line.strip())
            if m:
                val = (m.group(1) or "").strip().strip('"').strip("'")
                if not val or "change-me" in val.lower() or "sk-" in val.lower() or val == "your-secret-key":
                    new_val = secrets.token_hex(32)
                    lines.append(f"CHAT_JWT_SECRET={new_val}\n")
                    print(f"[deploy]  FIX: CHAT_JWT_SECRET was placeholder, generated persistent secret")
                else:
                    lines.append(line)
                found = True
            else:
                lines.append(line)
    if not found:
        new_val = secrets.token_hex(32)
        lines.append(f"CHAT_JWT_SECRET={new_val}\n")
        print(f"[deploy]  ADD: CHAT_JWT_SECRET (generated persistent secret)")
    with open(env_path, "w", encoding="utf-8") as f:
        f.writelines(lines)


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


def deploy_once(rel, dst, env_file):
    binary_asset = find_asset(rel, BINARY)
    client_asset = find_asset(rel, CLIENT_ARCHIVE)
    client_dst = os.path.join(CWD, CLIENT_ARCHIVE)
    with ThreadPoolExecutor(max_workers=2) as ex:
        f1 = ex.submit(download, binary_asset, dst)
        f2 = ex.submit(download, client_asset, client_dst)
        f1.result()
        f2.result()
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


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("cmd", nargs="?", default="all", choices=["download", "run", "all", "watch"])
    parser.add_argument("--proxy", default="", help="proxy URL for GitHub API (download uses gh-proxy.com)")
    parser.add_argument("--interval", type=int, default=120, help="poll interval in seconds (watch mode)")
    args = parser.parse_args()

    cmd = args.cmd
    proxy = args.proxy
    print(f"[deploy] command: {cmd}")
    if proxy:
        print(f"[deploy] proxy: {proxy}")
    print(f"[deploy] working dir: {CWD}")
    print(f"[deploy] binary: {BINARY}")
    setup_proxy(proxy)

    sync_env()

    dst = os.path.join(CWD, BINARY)
    env_file = os.path.join(CWD, ".env")

    if cmd in ("download", "all"):
        rel = latest_release()
        deploy_once(rel, dst, env_file)
        tag = rel["tag_name"]
        print(f"[deploy] download complete (tag: {tag})")
        write_known_tag(tag)

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
                ensure_jwt_secret(env_file)
            else:
                print(f"[deploy] no .env.example at {example}, skipping")
        env = load_env(env_file)
        os.chdir(CWD)
        print(f"[deploy] starting: {dst}")
        print(f"[deploy] --- server output below ---")
        subprocess.run([dst], env=env)

    if cmd == "watch":
        known = read_known_tag()
        print(f"[deploy] watch mode started (interval: {args.interval}s, known tag: {known})")
        while True:
            try:
                rel = latest_release()
                tag = rel["tag_name"]
                if tag != known:
                    print(f"[deploy] new release detected: {tag}")
                    kill_chatd()
                    deploy_once(rel, dst, env_file)
                    write_known_tag(tag)
                    known = tag
                    env = load_env(env_file)
                    start_detached(dst, env)
                    print(f"[deploy] deployed {tag}, sleeping {args.interval}s...")
                else:
                    print(f"[deploy] no new release (current: {tag})")
                time.sleep(args.interval)
            except KeyboardInterrupt:
                print("[deploy] watch mode stopped")
                break
            except Exception as e:
                print(f"[deploy] ERROR in watch loop: {e}")
                time.sleep(args.interval)

    if cmd not in ("download", "run", "all", "watch"):
        print(f"usage: python {sys.argv[0]} [download|run|all|watch]")
        sys.exit(1)


if __name__ == "__main__":
    main()
