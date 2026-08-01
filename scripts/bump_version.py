#!/usr/bin/env python3
"""版本 bump 脚本:一次同步三处版本号,防止手工遗漏。

用法:
    python3 scripts/bump_version.py 0.9.8            # 只改文件
    python3 scripts/bump_version.py 0.9.8 --commit   # 改文件 + git commit
    python3 scripts/bump_version.py 0.9.8 --tag      # 改文件 + commit + tag v0.9.8

同步位置:
    client/package.json                     version 字段
    server/internal/handlers/swagger.json   info.version(go:embed 权威)
    docs/api/reference.md                   /api/version 示例
"""
import argparse
import json
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SEMVER = re.compile(r"^\d+\.\d+\.\d+$")


def check_repo_clean():
    out = subprocess.run(["git", "status", "--porcelain"], capture_output=True, text=True).stdout.strip()
    if out:
        print("工作区不干净,拒绝执行(避免误提交并行工作):\n" + out)
        sys.exit(1)


def bump_package_json(version: str) -> bool:
    path = ROOT / "client" / "package.json"
    data = json.loads(path.read_text(encoding="utf-8"))
    old = data["version"]
    if old == version:
        return False
    data["version"] = version
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"client/package.json: {old} -> {version}")
    return True


def bump_swagger(version: str) -> bool:
    path = ROOT / "server" / "internal" / "handlers" / "swagger.json"
    data = json.loads(path.read_text(encoding="utf-8"))
    old = data["info"]["version"]
    if old == version:
        return False
    data["info"]["version"] = version
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"swagger.json info.version: {old} -> {version}")
    return True


def bump_reference(version: str) -> bool:
    path = ROOT / "docs" / "api" / "reference.md"
    text = path.read_text(encoding="utf-8")
    pattern = re.compile(r'\{\"version\": "\d+\.\d+\.\d+"\}')
    m = pattern.search(text)
    if not m:
        print(f"reference.md: 未找到版本示例,跳过")
        return False
    old = m.group(0)
    if old == f'{{"version": "{version}"}}':
        return False
    path.write_text(pattern.sub(f'{{"version": "{version}"}}', text), encoding="utf-8")
    print(f"docs/api/reference.md: {old} -> {version}")
    return True


def main():
    ap = argparse.ArgumentParser(description="同步三处版本号")
    ap.add_argument("version", help="新版本号,如 0.9.8")
    ap.add_argument("--commit", action="store_true", help="同时 git commit")
    ap.add_argument("--tag", action="store_true", help="同时 commit + tag v<version>")
    args = ap.parse_args()

    if not SEMVER.match(args.version):
        print(f"非法版本号: {args.version}(需 X.Y.Z)")
        sys.exit(1)
    if args.tag:
        args.commit = True
    if args.commit:
        check_repo_clean()

    changed = [
        bump_package_json(args.version),
        bump_swagger(args.version),
        bump_reference(args.version),
    ]
    if not any(changed):
        print("三处版本号已一致,无需修改")
        return
    if not args.commit:
        print("\n完成。可复查: git diff -- client/package.json "
              "server/internal/handlers/swagger.json docs/api/reference.md")
        return

    subprocess.run(["git", "add",
                    "client/package.json",
                    "server/internal/handlers/swagger.json",
                    "docs/api/reference.md"], check=True)
    subprocess.run(["git", "commit", "-m", f"chore: bump version to v{args.version}"], check=True)
    if args.tag:
        subprocess.run(["git", "tag", f"v{args.version}"], check=True)
    print(f"\n已提交{'(并打 tag v' + args.version + ')' if args.tag else ''}。")


if __name__ == "__main__":
    main()
