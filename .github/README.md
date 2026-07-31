# GitHub Actions — 踩坑记录

当前工作流：`.github/workflows/ci.yml`（后端测试 + 前端构建 + 交叉编译 + release）与 `frontend-ci.yml`（mock E2E + full E2E）。本文是 CI 配置过程中踩过的坑，历史有效。

## 目标

CI 需要完成：
- `go-test` — lint + unit test
- `frontend-build` — npm ci + build + Playwright
- `go-build` — 交叉编译 linux/amd64, linux/arm64, windows/amd64 三个二进制
- `release` — 收集三个二进制 + client/dist → 打包成一个 artifact

---

## 1. 初始问题：upload-artifact@v4 拒绝 `..` 路径

**现象**：`go-build` 三个 job 全部失败，日志显示：

```
Invalid pattern 'server/../chatd-linux-amd64'. Relative pathing '.' and '..' is not allowed.
```

**原因**：`actions/upload-artifact@v4` 出于安全考虑，不再允许路径中包含 `..`。

**当时配置**：build 命令 `go build -o ../chatd-linux-amd64 ./cmd/chatd/`，upload 的 path 写的是 `server/../chatd-linux-amd64`。

**解决**：build 的 `-o` 相对于 workspace root（`../` 之后），所以 upload path 直接写 `chatd-linux-amd64` 即可：

```yaml
path: chatd-${{ matrix.goos }}-${{ matrix.goarch }}${{ matrix.suffix }}
```

---

## 2. Windows 二进制名漏了 `.exe`

**现象**：`release` job 失败：

```
mv: cannot stat 'chatd-windows-amd64': No such file or directory
```

**原因**：upload 的 path 正确为 `chatd-windows-amd64.exe`，但 release 脚本里 `mv` 的目标名写的是 `chatd-windows-amd64` 没有 `.exe`。

**解决**：统一两边的名字：

```yaml
mv chatd-windows-amd64.exe release/chatd-windows-amd64.exe
```

---

## 3. client-dist 下载后找不到目录

**现象**：`release` job 中 `mv client-dist release/client-dist` 报找不到。

**原因**：前端 build 的 artifact 上传的是 `client/dist/` 下的文件（不是目录本身）。之前用 `download-artifact` + `merge-multiple: true` 会把所有 artifact 的文件平铺到 workspace root，不会保留目录名。

**解决**：分开下载：

1. `go-build` 的二进制用 `pattern: chatd-*` + `merge-multiple: true` → 全部放到 root
2. `client-dist` 单独下载到指定目录 `path: client-dist` → 文件存放在 `client-dist/` 下
3. 用 `cp -r` 而非 `mv`

```yaml
- uses: actions/download-artifact@v4
  with:
    name: client-dist
    path: client-dist
- run: cp -r client-dist/* release/client-dist/
```

---

## 最终成功

```
## 4. `gh release create` 返回 403

**现象**：`HTTP 403: Resource not accessible by integration`

**原因**：默认 `GITHUB_TOKEN` 没有 `contents: write` 权限。

**解决**：在 release job 上显式声明权限：

```yaml
release:
  permissions:
    contents: write
```

## 最终

```
go-test        ✅
frontend-build ✅
go-build (linux/amd64)   ✅
go-build (linux/arm64)   ✅
go-build (windows/amd64) ✅
release        ✅ → GitHub Release 自动创建，三个二进制已上传
```
```
