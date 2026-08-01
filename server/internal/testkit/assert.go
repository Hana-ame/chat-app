// Package testkit provides shared assertion helpers for backend tests.
//
// 本包是零依赖叶子包(只依赖标准库),任何包的 *_test.go 都可以直接 import,
// 不会产生 import cycle。testutil 包通过薄包装转发这些函数,保持既有调用
// (testutil.RequireXxx)不变。
package testkit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// formatMsg 将 msgAndArgs 渲染为错误消息:无参数返回空串;
// 单参数返回其字符串形式;多参数按 fmt.Sprintf 格式化。
func formatMsg(msgAndArgs []interface{}) string {
	switch len(msgAndArgs) {
	case 0:
		return ""
	case 1:
		return fmt.Sprint(msgAndArgs[0])
	default:
		return fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
	}
}

// Require* 断言助手:统一测试断言风格,失败即 t.Fatalf 终止当前测试。
// 约定:
//   - 所有断言失败都给出"期望 vs 实际"的完整上下文,方便直接定位;
//   - 断言 HTTP 响应状态的助手接收 *http.Response,并在失败时附带响应体前 512 字节,
//     以便快速看到后端返回的错误信息;
//   - 新测试一律使用这些助手,禁止再手写 if x != y { t.Fatalf(...) }。

// RequireNoError 断言 err 为 nil,否则终止测试。
func RequireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// RequireError 断言 err 非 nil,否则终止测试。
func RequireError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

// RequireErrorContains 断言 err 非 nil 且错误信息包含 substr,否则终止测试。
func RequireErrorContains(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", substr)
	}
	if !strings.Contains(err.Error(), substr) {
		t.Fatalf("error %q does not contain %q", err.Error(), substr)
	}
}

// RequireEqual 断言 got 与 want 深度相等(支持任意类型),否则终止测试。
func RequireEqual(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mismatch:\n got:  %#v\n want: %#v", got, want)
	}
}

// RequireNotEqual 断言 got 与 notWant 不相等,否则终止测试。
func RequireNotEqual(t *testing.T, got, notWant any) {
	t.Helper()
	if reflect.DeepEqual(got, notWant) {
		t.Fatalf("unexpected equal value: %#v", got)
	}
}

// RequireTrue 断言 cond 为 true,否则终止测试并输出 msgAndArgs(可选,
// 支持 fmt.Sprintf 风格格式串)。
func RequireTrue(t *testing.T, cond bool, msgAndArgs ...interface{}) {
	t.Helper()
	if !cond {
		t.Fatalf("expected true, got false: %s", formatMsg(msgAndArgs))
	}
}

// RequireFalse 断言 cond 为 false,否则终止测试并输出 msgAndArgs(可选,
// 支持 fmt.Sprintf 风格格式串)。
func RequireFalse(t *testing.T, cond bool, msgAndArgs ...interface{}) {
	t.Helper()
	if cond {
		t.Fatalf("expected false, got true: %s", formatMsg(msgAndArgs))
	}
}

// isNil 用 reflect 判断任意值是否 nil(含类型化 nil 指针/接口)。
func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Ptr, reflect.Slice:
		return rv.IsNil()
	}
	return false
}

// RequireNil 断言 v 为 nil(含类型化 nil 指针),否则终止测试。
func RequireNil(t *testing.T, v any) {
	t.Helper()
	if !isNil(v) {
		t.Fatalf("expected nil, got %#v", v)
	}
}

// RequireNotNil 断言 v 非 nil(含类型化 nil 指针),否则终止测试。
func RequireNotNil(t *testing.T, v any) {
	t.Helper()
	if isNil(v) {
		t.Fatalf("expected non-nil, got nil")
	}
}

// RequireStatus 断言 HTTP 响应状态码 == want,失败时附带响应体前 512 字节。
func RequireStatus(t *testing.T, res *http.Response, want int) {
	t.Helper()
	if res.StatusCode != want {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		t.Fatalf("status mismatch: got %d, want %d, body: %s", res.StatusCode, want, body)
	}
}

// RequireJSONBody 将响应体解码为 JSON,成功即返回,失败则终止测试。
// 调用方负责在之后 close 响应体。
func RequireJSONBody(t *testing.T, res *http.Response, into any) {
	t.Helper()
	body, err := io.ReadAll(res.Body)
	RequireNoError(t, err)
	if err := json.Unmarshal(body, into); err != nil {
		t.Fatalf("invalid json response: %v, body: %s", err, body)
	}
}

// RequireBodyContains 断言响应体包含 substr,失败时输出完整响应体。
func RequireBodyContains(t *testing.T, res *http.Response, substr string) {
	t.Helper()
	body, err := io.ReadAll(res.Body)
	RequireNoError(t, err)
	if !strings.Contains(string(body), substr) {
		t.Fatalf("response body does not contain %q: %s", substr, body)
	}
}

// RequireContains 断言字符串 s 包含 substr,否则终止测试。
func RequireContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Fatalf("string %q does not contain %q", s, substr)
	}
}

// RequireLen 断言容器(v 为 reflect.Len 可用的切片/数组/字符串/map)
// 长度 == want,否则终止测试。
func RequireLen(t *testing.T, v any, want int) {
	t.Helper()
	got := reflect.ValueOf(v).Len()
	if got != want {
		t.Fatalf("length mismatch: got %d, want %d (%#v)", got, want, v)
	}
}

// RequireStatusAny 断言 HTTP 响应状态码属于 wants 之一,失败时附带
// 响应体前 512 字节。用于防探测设计的端点(如 403/404 均可的越权场景)。
func RequireStatusAny(t *testing.T, res *http.Response, wants ...int) {
	t.Helper()
	for _, w := range wants {
		if res.StatusCode == w {
			return
		}
	}
	body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
	t.Fatalf("status mismatch: got %d, want one of %v, body: %s", res.StatusCode, wants, body)
}

// RequireJSONError 断言响应体包含 "error":"<errCode>",失败时输出完整响应体。
func RequireJSONError(t *testing.T, res *http.Response, errCode string) {
	t.Helper()
	body, err := io.ReadAll(res.Body)
	RequireNoError(t, err)
	if !strings.Contains(string(body), `"error":"`+errCode+`"`) {
		t.Fatalf("response body has no error %q: %s", errCode, body)
	}
}
