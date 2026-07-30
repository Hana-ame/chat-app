package logutil

import (
	"bytes"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func captureOutput(fn func()) string {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	fn()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	return buf.String()
}

func TestLevelFiltering(t *testing.T) {
	tests := []struct {
		name      string
		setLevel  Level
		callLevel Level
		expectLog bool
	}{
		{"DEBUG when DEBUG", DEBUG, DEBUG, true},
		{"DEBUG when INFO", INFO, DEBUG, false},
		{"INFO when INFO", INFO, INFO, true},
		{"INFO when WARN", WARN, INFO, false},
		{"WARN when WARN", WARN, WARN, true},
		{"WARN when ERROR", ERROR, WARN, false},
		{"ERROR when ERROR", ERROR, ERROR, true},
		{"ERROR when DEBUG", DEBUG, ERROR, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			currentLevel = tc.setLevel
			out := captureOutput(func() {
				output(2, tc.callLevel, "test %s", "msg")
			})
			hasLog := strings.Contains(out, "test msg")
			if hasLog != tc.expectLog {
				t.Fatalf("currentLevel=%d callLevel=%d: expectLog=%v got=%q",
					tc.setLevel, tc.callLevel, tc.expectLog, out)
			}
		})
	}
}

func TestOutputLineFormat(t *testing.T) {
	currentLevel = DEBUG
	out := captureOutput(func() {
		output(1, INFO, "hello %d", 42)
	})
	if !strings.Contains(out, "level=INFO") {
		t.Fatalf("missing level: %s", out)
	}
	if !strings.Contains(out, "hello 42") {
		t.Fatalf("missing message: %s", out)
	}
	if !strings.Contains(out, "log_test.go") {
		t.Fatalf("missing caller location: %s", out)
	}
}

func TestDebugInfoWarnError(t *testing.T) {
	currentLevel = DEBUG
	out := captureOutput(func() {
		Debug("debug msg")
		Info("info msg")
		Warn("warn msg")
		Error("error msg")
	})
	for _, s := range []string{"level=DEBUG", "level=INFO", "level=WARN", "level=ERROR"} {
		if !strings.Contains(out, s) {
			t.Fatalf("missing %s in output: %s", s, out)
		}
	}
	for _, s := range []string{"debug msg", "info msg", "warn msg", "error msg"} {
		if !strings.Contains(out, s) {
			t.Fatalf("missing %q in output: %s", s, out)
		}
	}
}

func TestRuntimeCallerFallback(t *testing.T) {
	currentLevel = DEBUG
	out := captureOutput(func() {
		output(99, INFO, "fallback")
	})
	if !strings.Contains(out, "???:0") {
		t.Fatalf("expected fallback, got: %s", out)
	}
}

func TestInitDefaultLevel(t *testing.T) {
	os.Unsetenv("LOG_LEVEL")
	loadLevel()
	if currentLevel != INFO {
		t.Fatalf("default level should be INFO, got %d", currentLevel)
	}
}

func TestInitDebugLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "DEBUG")
	loadLevel()
	if currentLevel != DEBUG {
		t.Fatalf("want DEBUG, got %d", currentLevel)
	}
}

func TestInitWarnLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "WARN")
	loadLevel()
	if currentLevel != WARN {
		t.Fatalf("want WARN, got %d", currentLevel)
	}
}

func TestInitErrorLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "ERROR")
	loadLevel()
	if currentLevel != ERROR {
		t.Fatalf("want ERROR, got %d", currentLevel)
	}
}

func TestInitCaseInsensitive(t *testing.T) {
	t.Setenv("LOG_LEVEL", "debug")
	loadLevel()
	if currentLevel != DEBUG {
		t.Fatalf("want DEBUG, got %d", currentLevel)
	}
}

func TestInitInvalidLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "invalid")
	loadLevel()
	if currentLevel != INFO {
		t.Fatalf("want INFO fallback, got %d", currentLevel)
	}
}

func TestInitEmptyLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "")
	loadLevel()
	if currentLevel != INFO {
		t.Fatalf("want INFO for empty, got %d", currentLevel)
	}
}

func TestFatal(t *testing.T) {
	currentLevel = ERROR
	exited := false
	osExit = func(code int) {
		exited = true
		if code != 1 {
			t.Fatalf("want exit code 1, got %d", code)
		}
	}
	defer func() { osExit = os.Exit }()

	out := captureOutput(func() {
		Fatal("fatal %s", "err")
	})
	if !exited {
		t.Fatal("Fatal should call osExit")
	}
	if !strings.Contains(out, "fatal err") {
		t.Fatalf("missing fatal message: %s", out)
	}
}

func TestFatalAlwaysExits(t *testing.T) {
	currentLevel = ERROR
	called := false
	osExit = func(code int) { called = true }
	defer func() { osExit = os.Exit }()

	Fatal("should exit")
	if !called {
		t.Fatal("Fatal should always call osExit")
	}
}
