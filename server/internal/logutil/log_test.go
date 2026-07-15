package logutil

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger = log.New(&buf, "", 0)

	tests := []struct {
		name       string
		setLevel   Level
		callLevel  Level
		expectLog  bool
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
			buf.Reset()
			currentLevel = tc.setLevel
			output(2, tc.callLevel, "test %s", "msg")
			got := buf.String()
			hasLog := strings.Contains(got, "test msg")
			if hasLog != tc.expectLog {
				t.Fatalf("currentLevel=%d callLevel=%d: expectLog=%v got=%q",
					tc.setLevel, tc.callLevel, tc.expectLog, got)
			}
		})
	}
}

func TestOutputLineFormat(t *testing.T) {
	var buf bytes.Buffer
	logger = log.New(&buf, "", 0)
	currentLevel = DEBUG
	buf.Reset()

	output(1, INFO, "hello %d", 42)
	line := buf.String()

	if !strings.Contains(line, "[INFO]") {
		t.Fatalf("missing level tag: %s", line)
	}
	if !strings.Contains(line, "log_test.go") {
		t.Fatalf("missing caller file: %s", line)
	}
	if !strings.Contains(line, "hello 42") {
		t.Fatalf("missing message: %s", line)
	}
}

func TestDebugInfoWarnError(t *testing.T) {
	var buf bytes.Buffer
	logger = log.New(&buf, "", 0)
	currentLevel = DEBUG
	buf.Reset()

	Debug("debug msg")
	Info("info msg")
	Warn("warn msg")
	Error("error msg")
	out := buf.String()

	for _, s := range []string{"[DEBUG]", "[INFO]", "[WARN]", "[ERROR]"} {
		if !strings.Contains(out, s) {
			t.Fatalf("missing %s in output", s)
		}
	}
}

func TestRuntimeCallerFallback(t *testing.T) {
	var buf bytes.Buffer
	logger = log.New(&buf, "", 0)
	currentLevel = DEBUG
	buf.Reset()

	// output with a calldepth that will fail runtime.Caller
	output(99, INFO, "fallback")
	line := buf.String()
	if !strings.Contains(line, "???:0:") {
		t.Fatalf("expected fallback format, got: %s", line)
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
	var buf bytes.Buffer
	logger = log.New(&buf, "", 0)
	currentLevel = ERROR

	exited := false
	osExit = func(code int) {
		exited = true
		if code != 1 {
			t.Fatalf("want exit code 1, got %d", code)
		}
	}
	defer func() { osExit = os.Exit }()

	Fatal("fatal %s", "err")
	if !exited {
		t.Fatal("Fatal should call osExit")
	}
	if !strings.Contains(buf.String(), "fatal err") {
		t.Fatalf("missing fatal message: %s", buf.String())
	}
}

func TestFatalLevelFiltered(t *testing.T) {
	var buf bytes.Buffer
	logger = log.New(&buf, "", 0)
	currentLevel = ERROR

	called := false
	osExit = func(code int) { called = true }
	defer func() { osExit = os.Exit }()

	Fatal("should output")
	if !called {
		t.Fatal("Fatal should always call osExit")
	}
}
