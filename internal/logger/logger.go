package logger

import (
	"fmt"
	"os"
	"time"
)

type Level string

const (
	LevelInfo  Level = "INFO"
	LevelDebug Level = "DEBUG"
	LevelError Level = "ERROR"
)

// Entry — одна запись лога (передаётся в UI).
type Entry struct {
	Time    string `json:"time"`
	Level   Level  `json:"level"`
	Message string `json:"message"`
}

// Logger — логгер с колбэком в UI и записью в файл.
type Logger struct {
	onEntry func(Entry)
	file    *os.File
}

func New(onEntry func(Entry)) *Logger {
	f, _ := os.OpenFile("furor_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	return &Logger{onEntry: onEntry, file: f}
}

func (l *Logger) log(level Level, msg string) {
	entry := Entry{
		Time:    time.Now().Format("15:04:05"),
		Level:   level,
		Message: msg,
	}
	if l.onEntry != nil {
		l.onEntry(entry)
	}
	if l.file != nil {
		fmt.Fprintf(l.file, "[%s] %s %s\n", entry.Time, level, msg)
	}
}

func (l *Logger) Info(msg string)                        { l.log(LevelInfo, msg) }
func (l *Logger) Debug(msg string)                       { l.log(LevelDebug, msg) }
func (l *Logger) Error(msg string)                       { l.log(LevelError, msg) }
func (l *Logger) Infof(f string, args ...interface{})    { l.log(LevelInfo, fmt.Sprintf(f, args...)) }
func (l *Logger) Debugf(f string, args ...interface{})   { l.log(LevelDebug, fmt.Sprintf(f, args...)) }
func (l *Logger) Errorf(f string, args ...interface{})   { l.log(LevelError, fmt.Sprintf(f, args...)) }
