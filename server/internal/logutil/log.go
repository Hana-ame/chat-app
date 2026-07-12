package logutil

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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

var (
	currentLevel Level
	logger       *log.Logger
	mu           sync.Mutex
)

func init() {
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
	logger = log.New(os.Stderr, "", 0)
}

func output(calldepth int, level Level, format string, args ...interface{}) {
	if level < currentLevel {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	_, file, line, ok := runtime.Caller(calldepth)
	if !ok {
		file = "???"
		line = 0
	}
	short := filepath.Base(file)
	msg := fmt.Sprintf(format, args...)
	logger.Printf("[%s] %s:%d: %s", levelNames[level], short, line, msg)
}

func Debug(format string, args ...interface{}) { output(2, DEBUG, format, args...) }
func Info(format string, args ...interface{})  { output(2, INFO, format, args...) }
func Warn(format string, args ...interface{})  { output(2, WARN, format, args...) }
func Error(format string, args ...interface{}) { output(2, ERROR, format, args...) }
func Fatal(format string, args ...interface{}) {
	output(2, ERROR, format, args...)
	os.Exit(1)
}
