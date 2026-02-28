// Package logger provides a minimal levelled logger whose output format mirrors
// the Quarkus pattern  "%d{yyyy-MM-dd'T'HH:mm:ss.SSSXXX} %5p … %m%n":
//
//	2026-02-28T09:52:49.123+08:00  INFO [stop] session 78da92ac teardown complete
//	2026-02-28T09:52:49.123+08:00 ERROR [prepare] cleanup: stop channel: dial tcp …
package logger

import (
	"fmt"
	"os"
	"time"
)

// timeFormat produces ISO-8601 timestamps with milliseconds and timezone offset,
// e.g. 2026-02-28T09:52:49.123+08:00.
const timeFormat = "2006-01-02T15:04:05.000Z07:00"

func write(level, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	ts := time.Now().Format(timeFormat)
	// %5s right-justifies the level label to 5 characters (matches Quarkus %5p).
	fmt.Fprintf(os.Stdout, "%s %5s %s\n", ts, level, msg)
}

// Infof logs a message at INFO level.
func Infof(format string, args ...any) {
	write("INFO", format, args...)
}

// Warnf logs a message at WARN level.
func Warnf(format string, args ...any) {
	write("WARN", format, args...)
}

// Errorf logs a message at ERROR level.
func Errorf(format string, args ...any) {
	write("ERROR", format, args...)
}

// Fatalf logs a message at ERROR level then exits with status 1.
func Fatalf(format string, args ...any) {
	write("ERROR", format, args...)
	os.Exit(1)
}
