// Package local_test 覆盖 storage/local 上传驱动。
//
// 测试范围:
//   - Put/Get/Head/Delete 完整生命周期(含 ETag 与 content-type 推导)
//   - 路径穿越防护(.. 与绝对路径)
//   - 文件不存在 / 删除幂等
//   - SafeContentType 内容类型白名单(XSS 防护)
//
// 运行方式: cd server && go test ./internal/storage/local/
// 说明:全部使用 t.TempDir() 临时目录,不触碰真实 uploads/。
package local_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/Hana-ame/chat-app/server/internal/storage/local"
	"github.com/Hana-ame/chat-app/server/internal/testutil"
)

// 测试辅助:创建临时驱动。root 为 t.TempDir() 子目录,确保每个用例独立。
func newDriver(t *testing.T) *local.Driver {
	t.Helper()
	d, err := local.New(t.TempDir())
	testutil.RequireNoError(t, err)
	return d
}

func TestPutGetRoundtrip(t *testing.T) {
	// Put 写入后 Get 应能原样读回,size 与 content-type 与文件名推导一致。
	d := newDriver(t)

	res, err := d.Put("2026/08/01/hello.txt", "text/plain", strings.NewReader("hello world"))
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, res.ID, "2026/08/01/hello.txt")

	body, size, contentType, err := d.Get("2026/08/01/hello.txt")
	testutil.RequireNoError(t, err)
	defer body.Close()

	data, err := io.ReadAll(body)
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, string(data), "hello world")
	testutil.RequireEqual(t, size, int64(11))
	testutil.RequireEqual(t, contentType, "text/plain")
}

func TestPutETagIsSHA256(t *testing.T) {
	// ETag 必须是文件内容的 sha256,且带引号包裹(HTTP 标准格式)。
	d := newDriver(t)

	res, err := d.Put("a.bin", "application/octet-stream", strings.NewReader("abc"))
	testutil.RequireNoError(t, err)
	testutil.RequireTrue(t, len(res.ETag) == 66 && strings.HasPrefix(res.ETag, `"`), "etag should be quoted sha256: "+res.ETag)

	// 相同内容应得到相同 ETag(去重判断依据)。
	res2, err := d.Put("b.bin", "application/octet-stream", strings.NewReader("abc"))
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, res.ETag, res2.ETag)
}

func TestPutRejectsTraversal(t *testing.T) {
	// .. 与绝对路径必须在写入前被拒绝,防止逃逸 root 目录。
	d := newDriver(t)

	for _, p := range []string{"../evil.txt", "../../etc/passwd", "/etc/passwd", "a/../../b.txt", `a/..\b.txt`} {
		_, err := d.Put(p, "text/plain", strings.NewReader("x"))
		testutil.RequireError(t, err)
	}
}

func TestGetRejectsTraversal(t *testing.T) {
	// 读取路径同样受穿越防护。
	d := newDriver(t)

	for _, p := range []string{"../evil.txt", "/etc/passwd"} {
		_, _, _, err := d.Get(p)
		testutil.RequireError(t, err)
		_, _, err = d.Head(p)
		testutil.RequireError(t, err)
	}
}

func TestGetNotFound(t *testing.T) {
	// 不存在的文件应返回"not found"错误而非其他错误。
	d := newDriver(t)

	_, _, _, err := d.Get("nope.txt")
	testutil.RequireErrorContains(t, err, "not found")
}

func TestDeleteLifecycle(t *testing.T) {
	// Delete 删除后 Get 报 not found;重复 Delete 应幂等(不报错)。
	d := newDriver(t)

	_, err := d.Put("x.txt", "text/plain", strings.NewReader("x"))
	testutil.RequireNoError(t, err)

	testutil.RequireNoError(t, d.Delete("x.txt"))
	_, _, _, err = d.Get("x.txt")
	testutil.RequireErrorContains(t, err, "not found")

	testutil.RequireNoError(t, d.Delete("x.txt"))
}

func TestHead(t *testing.T) {
	// Head 只返回元数据,不读取文件体。
	d := newDriver(t)

	_, err := d.Put("img.png", "image/png", bytes.NewReader(make([]byte, 100)))
	testutil.RequireNoError(t, err)

	size, contentType, err := d.Head("img.png")
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, size, int64(100))
	testutil.RequireEqual(t, contentType, "image/png")

	_, _, err = d.Head("missing.png")
	testutil.RequireErrorContains(t, err, "not found")
}

func TestPutCreatesNestedDirectories(t *testing.T) {
	// Put 应自动创建多层目录结构(时间戳分区路径)。
	d := newDriver(t)

	_, err := d.Put("2026/07/31/deep/nested/file.txt", "text/plain", strings.NewReader("deep"))
	testutil.RequireNoError(t, err)

	body, _, _, err := d.Get("2026/07/31/deep/nested/file.txt")
	testutil.RequireNoError(t, err)
	defer body.Close()
	data, _ := io.ReadAll(body)
	testutil.RequireEqual(t, string(data), "deep")
}

// SafeContentType 白名单表:允许内联渲染 vs 强制下载(application/octet-stream)。
// 危险类型(HTML/SVG/XML/JS 等)必须被降级,防止同源存储型 XSS。
func TestSafeContentType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		want     string
		wantSafe bool
	}{
		{"empty becomes octet-stream", "", "application/octet-stream", false},
		{"octet-stream passthrough", "application/octet-stream", "application/octet-stream", true},
		{"text/plain allowed", "text/plain", "text/plain", true},
		{"pdf allowed", "application/pdf", "application/pdf", true},
		{"png allowed", "image/png", "image/png", true},
		{"svg blocked", "image/svg+xml", "application/octet-stream", false},
		{"html blocked", "text/html", "application/octet-stream", false},
		{"javascript blocked", "application/javascript", "application/octet-stream", false},
		{"xml blocked", "application/xml", "application/octet-stream", false},
		{"video allowed", "video/mp4", "video/mp4", true},
		{"audio allowed", "audio/mpeg", "audio/mpeg", true},
		{"unknown blocked", "application/x-unknown", "application/octet-stream", false},
		{"params stripped", "text/plain; charset=utf-8", "text/plain", true},
		{"case normalized", "IMAGE/PNG", "image/png", true},
		{"spaces trimmed", "  image/gif  ", "image/gif", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, safe := local.SafeContentType(tc.input)
			testutil.RequireEqual(t, got, tc.want)
			testutil.RequireEqual(t, safe, tc.wantSafe)
		})
	}
}
