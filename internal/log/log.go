// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/apex/log"
)

var traceEnabled bool

// InitLogger sets up Apex with a custom handler and a log level from the
// TFQUERY_LOG env variable.
func InitLogger() {
	envLevel := strings.ToLower(os.Getenv("TFQUERY_LOG"))
	if envLevel == "" {
		envLevel = "error"
	}
	traceEnabled = envLevel == "trace"
	var apexLevel log.Level
	switch envLevel {
	case "trace":
		apexLevel = log.DebugLevel // Show debug and above for trace
	case "debug":
		apexLevel = log.DebugLevel
	case "info":
		apexLevel = log.InfoLevel
	case "warn":
		apexLevel = log.WarnLevel
	case "error":
		apexLevel = log.ErrorLevel
	case "fatal":
		apexLevel = log.FatalLevel
	default:
		apexLevel = log.ErrorLevel
	}
	log.SetHandler(new(CustomHandler))
	log.SetLevel(apexLevel)
}

// CustomHandler formats log messages and writes to stdout
type CustomHandler struct{}

// HandleLog implements the log.Handler interface
func (h *CustomHandler) HandleLog(e *log.Entry) error {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := e.Message
	level := "?"
	if strings.HasPrefix(message, "TRACE: ") {
		level = "T"
		message = message[7:]
	} else {
		switch e.Level {
		case log.DebugLevel:
			level = "D"
		case log.InfoLevel:
			level = "I"
		case log.WarnLevel:
			level = "W"
		case log.ErrorLevel:
			level = "E"
		case log.FatalLevel:
			level = "F"
		}
	}
	fmt.Fprintf(os.Stdout, "%s %s %s\n", timestamp, level, message)
	return nil
}

// Tracef logs at Trace level (below Debug).
func Tracef(format string, args ...interface{}) {
	if traceEnabled {
		log.Debug("TRACE: " + fmt.Sprintf(format, args...))
	}
}

// Debugf logs at Debug level.
func Debugf(format string, args ...interface{}) {
	log.Debugf(format, args...)
}

// Infof logs at Info level.
func Infof(format string, args ...interface{}) {
	log.Infof(format, args...)
}

// Errorf logs at Error level.
func Errorf(format string, args ...interface{}) {
	log.Errorf(format, args...)
}

// Debug logs at Debug level.
func Debug(msg string) {
	log.Debug(msg)
}

// Warnf logs at Warn level.
func Warnf(format string, args ...interface{}) {
	log.Warn(fmt.Sprintf(format, args...))
}

// WithError returns an entry with error.
func WithError(err error) *log.Entry {
	return log.WithError(err)
}
