import json
import os
import shutil
import subprocess
import sys
import urllib.request

REPO = "Hana-ame/chat-app"
BINARY = "chatd-windows-amd64.exe"
CWD = os.getcwd()


def latest_release():
    url = f"https://api.github.com/repos/{REPO}/releases"
    req = urllib.request.Request(url, headers={"User-Agent": "deploy.py"})
    with urllib.request.urlopen(req) as r:
        releases = json.loads(r.read())
    if not releases:
        print("no releases found")
        sys.exit(1)
    return releases[0]


def find_asset(release):
    for a in release.get("assets", []):
        if a["name"] == BINARY:
            return a
    print(f"asset {BINARY} not found in release {release['tag_name']}")
    sys.exit(1)


def download(asset, dst):
    url = asset["browser_download_url"]
    print(f"downloading {url}")
    if os.path.isfile(dst):
        os.remove(dst)
    subprocess.run(["curl.exe", "-sLo", dst, url], check=True)
    print(f"saved to {dst}")


def load_env(env_file):
    env = os.environ.copy()
    if env_file and os.path.isfile(env_file):
        print(f"loading env from {env_file}")
        with open(env_file) as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#") or "=" not in line:
                    continue
                k, _, v = line.partition("=")
                env[k.strip()] = v.strip()
    return env


def main():
    cmd = sys.argv[1] if len(sys.argv) > 1 else "all"

    dst = os.path.join(CWD, BINARY)
    env_file = os.path.join(CWD, ".env")

    if cmd in ("download", "all"):
        rel = latest_release()
        a = find_asset(rel)
        download(a, dst)
        print(f"tag: {rel['tag_name']}")

    if cmd in ("run", "all"):
        if not os.path.isfile(dst):
            print(f"{dst} not found, run deploy.py download first")
            sys.exit(1)
        if not os.path.isfile(env_file):
            example = os.path.join(CWD, ".env.example")
            if os.path.isfile(example):
                shutil.copy2(example, env_file)
                print(f"created {env_file} from .env.example")
        env = load_env(env_file)
        os.chdir(CWD)
        subprocess.run([dst], env=env)

    if cmd not in ("download", "run", "all"):
        print(f"usage: python {sys.argv[0]} [download|run|all]")
        sys.exit(1)


if __name__ == "__main__":
    main()

