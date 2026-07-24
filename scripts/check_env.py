import os
import sys
import re
import urllib.request

REPO_BASE = "https://raw.githubusercontent.com/Hana-ame/chat-app/main"

ENV_DIRS = [
    ("Root", ".env.example", ".env"),
    ("Server", "server/.env.example", "server/.env"),
    ("Client", "client/.env.example", "client/.env"),
]


def _parse(path):
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


def _fetch(rel_path):
    url = f"{REPO_BASE}/{rel_path}"
    try:
        resp = urllib.request.urlopen(url, timeout=10)
        return resp.read().decode("utf-8")
    except Exception as e:
        print(f"  [WARN] fetch failed: {e}", file=sys.stderr)
        return None


def check(display, example_rel, env_rel):
    root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    example_path = os.path.join(root, example_rel)
    env_path = os.path.join(root, env_rel)

    print(f"\n=== {display} ===")

    example_keys = _parse(example_path)
    if example_keys is None:
        print(f"  [WARN] {example_rel} not found locally, fetching from GitHub...")
        raw = _fetch(example_rel)
        if raw is None:
            print(f"  [FAIL] cannot obtain .env.example for {display}")
            return True
        with open(example_path, "w") as f:
            f.write(raw)
        print(f"  [OK] downloaded {example_rel}")
        example_keys = _parse(example_path)
        if example_keys is None:
            print(f"  [FAIL] parsed .env.example is empty for {display}")
            return True

    env_keys = _parse(env_path)
    if env_keys is None:
        print(f"  [WARN] {env_rel} not found")
        raw = _fetch(example_rel)
        if raw:
            with open(env_path, "w") as f:
                f.write(raw)
            print(f"  [INFO] created {env_rel} from GitHub (edit secrets manually)")
        else:
            print(f"  [INFO] copy {example_rel} to {env_rel} and fill in secrets")
        return False

    missing = []
    placeholder = []
    for k, v in example_keys.items():
        if k not in env_keys:
            missing.append(k)
        elif v is not None and env_keys.get(k) == v and (
            "change-me" in v.lower() or v.strip() == ""
        ):
            placeholder.append(k)

    if missing:
        print(f"  [WARN] missing keys: {', '.join(missing)}")
        raw = _fetch(example_rel)
        if raw:
            with open(env_path, "w") as f:
                f.write(raw)
            print(f"  [INFO] replaced {env_rel} with latest template (edit secrets)")
        return False

    if placeholder:
        print(f"  [WARN] placeholder values: {', '.join(placeholder)}")
        print(f"  [INFO] update these in {env_rel}")

    if not missing:
        print("  [OK] all env vars present")
    return True


if __name__ == "__main__":
    ok = True
    for display, example_rel, env_rel in ENV_DIRS:
        if not check(display, example_rel, env_rel):
            ok = False
    sys.exit(0 if ok else 1)
