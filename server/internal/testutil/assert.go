// Package testutil provides shared helpers for backend tests.
//
// 断言助手(Require*)实际定义在零依赖的 internal/testkit 包,本文件只做
// 薄转发,保持 testutil.RequireXxx 既有调用方式不变。
// 注意:testkit 是给所有包(包括 handlers 内部测试)用的;
// testutil 因 import 了 handlers,只能被不依赖 handlers 的包使用。
package testutil

import (
	"net/http"
	"testing"

	"github.com/Hana-ame/chat-app/server/internal/testkit"
)

func RequireNoError(t *testing.T, err error) { testkit.RequireNoError(t, err) }
func RequireError(t *testing.T, err error)   { testkit.RequireError(t, err) }
func RequireErrorContains(t *testing.T, err error, substr string) {
	testkit.RequireErrorContains(t, err, substr)
}
func RequireEqual(t *testing.T, got, want any)       { testkit.RequireEqual(t, got, want) }
func RequireNotEqual(t *testing.T, got, notWant any) { testkit.RequireNotEqual(t, got, notWant) }
func RequireTrue(t *testing.T, cond bool, msgAndArgs ...interface{}) {
	testkit.RequireTrue(t, cond, msgAndArgs...)
}
func RequireFalse(t *testing.T, cond bool, msgAndArgs ...interface{}) {
	testkit.RequireFalse(t, cond, msgAndArgs...)
}
func RequireNil(t *testing.T, v any)    { testkit.RequireNil(t, v) }
func RequireNotNil(t *testing.T, v any) { testkit.RequireNotNil(t, v) }
func RequireStatus(t *testing.T, res *http.Response, want int) {
	testkit.RequireStatus(t, res, want)
}
func RequireStatusAny(t *testing.T, res *http.Response, wants ...int) {
	testkit.RequireStatusAny(t, res, wants...)
}
func RequireJSONBody(t *testing.T, res *http.Response, into any) {
	testkit.RequireJSONBody(t, res, into)
}
func RequireBodyContains(t *testing.T, res *http.Response, substr string) {
	testkit.RequireBodyContains(t, res, substr)
}
