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
BINARY = "chatd-windows-amd64.exe"
CLIENT_ARCHIVE = "client-dist.tar.gz"
CWD = os.getcwd()


def setup_proxy(proxy):
    if not proxy:
        return
    os.environ.setdefault("HTTPS_PROXY", proxy)
    os.environ.setdefault("HTTP_PROXY", proxy)
    handler = ProxyHandler({"http": proxy, "https": proxy})
    install_opener(build_opener(handler))


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


REPO_BASE = "https://raw.githubusercontent.com/Hana-ame/chat-app/main"

EXAMPLE_FILES = [
    (".env.example", ".env"),
    ("server/.env.example", "server/.env"),
    ("client/.env.example", "client/.env"),
]


def _parse_env_file(path):
    keys = {}
    try:
        with open(path) as f:
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


def _fetch_from_repo(rel_path):
    url = f"{REPO_BASE}/{rel_path}"
    try:
        with urllib.request.urlopen(url, timeout=10) as r:
            return r.read().decode("utf-8")
    except Exception as e:
        print(f"[deploy]  WARN: fetch {rel_path} failed: {e}")
        return None


def check_env(cwd):
    ok = True
    for example_rel, env_rel in EXAMPLE_FILES:
        example_path = os.path.join(cwd, example_rel)
        env_path = os.path.join(cwd, env_rel)

        example_keys = _parse_env_file(example_path)
        if example_keys is None:
            print(f"[deploy]  WARN: {example_rel} not found, fetching from GitHub...")
            raw = _fetch_from_repo(example_rel)
            if raw:
                with open(example_path, "w") as f:
                    f.write(raw)
                print(f"[deploy]  OK: downloaded {example_rel}")
                example_keys = _parse_env_file(example_path)
            if example_keys is None:
                print(f"[deploy]  SKIP: cannot check {env_rel} (no example)")
                continue

        env_keys = _parse_env_file(env_path)
        if env_keys is None:
            print(f"[deploy]  WARN: {env_rel} not found")
            raw = _fetch_from_repo(example_rel)
            if raw:
                with open(env_path, "w") as f:
                    f.write(raw)
                print(f"[deploy]  INFO: created {env_rel} from GitHub template")
                print(f"[deploy]  INFO: edit {env_rel} to set secrets before running")
            else:
                print(f"[deploy]  INFO: copy {example_rel} to {env_rel} and fill in secrets")
            ok = False
            continue

        missing = []
        placeholder = []
        for k, v in example_keys.items():
            if k not in env_keys:
                missing.append(k)
            elif v is not None and env_keys.get(k) == v and (
                "change-me" in v.lower() or "sk-" in v.lower() or v.strip() == ""
            ):
                placeholder.append(k)

        if missing:
            print(f"[deploy]  WARN: {env_rel} missing keys: {', '.join(missing)}")
            raw = _fetch_from_repo(example_rel)
            if raw:
                with open(env_path, "w") as f:
                    f.write(raw)
                print(f"[deploy]  INFO: replaced {env_rel} with latest template (edit secrets)")
            ok = False
        elif placeholder:
            print(f"[deploy]  WARN: {env_rel} placeholder values: {', '.join(placeholder)}")
            print(f"[deploy]  INFO: update these in {env_rel}")
        else:
            print(f"[deploy]  OK: {env_rel} looks good")
    return ok


def load_env(env_file):
    env = os.environ.copy()
    if env_file and os.path.isfile(env_file):
        print(f"[deploy] loading env from: {env_file}")
        with open(env_file) as f:
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
    args = parser.parse_args()

    cmd = args.cmd
    proxy = args.proxy
    print(f"[deploy] command: {cmd}")
    print(f"[deploy] proxy: {proxy if proxy else '(none)'}")
    print(f"[deploy] working dir: {CWD}")
    print(f"[deploy] binary: {BINARY}")
    setup_proxy(proxy)

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
        # Write CHAT_STATIC_DIR as absolute path so it works regardless of working directory
        env_file = os.path.join(CWD, ".env")
        static_line = f"CHAT_STATIC_DIR={client_dir}"
        if os.path.isfile(env_file):
            with open(env_file) as f:
                lines = f.readlines()
            with open(env_file, "w") as f:
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
            with open(env_file, "a") as f:
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

