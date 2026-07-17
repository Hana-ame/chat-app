package logutil

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

var levelNames = map[Level]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
}

var levelMap = map[Level]slog.Level{
	DEBUG: slog.LevelDebug,
	INFO:  slog.LevelInfo,
	WARN:  slog.LevelWarn,
	ERROR: slog.LevelError,
}

var (
	currentLevel Level
	osExit       = os.Exit
)

func init() {
	loadLevel()
}

func loadLevel() {
	lvl := os.Getenv("LOG_LEVEL")
	switch strings.ToUpper(lvl) {
	case "DEBUG":
		currentLevel = DEBUG
	case "INFO", "":
		currentLevel = INFO
	case "WARN":
		currentLevel = WARN
	case "ERROR":
		currentLevel = ERROR
	default:
		currentLevel = INFO
	}
}

func output(calldepth int, level Level, format string, args ...interface{}) {
	if level < currentLevel {
		return
	}
	_, file, line, ok := runtime.Caller(calldepth)
	if !ok {
		file = "???"
		line = 0
	}
	msg := fmt.Sprintf(format, args...)
	slog.Log(context.Background(), levelMap[level], msg,
		"source", fmt.Sprintf("%s:%d", filepath.Base(file), line),
	)
}

func Debug(format string, args ...interface{}) { output(2, DEBUG, format, args...) }
func Info(format string, args ...interface{})  { output(2, INFO, format, args...) }
func Warn(format string, args ...interface{})  { output(2, WARN, format, args...) }
func Error(format string, args ...interface{}) { output(2, ERROR, format, args...) }
func Fatal(format string, args ...interface{}) {
	output(2, ERROR, format, args...)
	osExit(1)
}
