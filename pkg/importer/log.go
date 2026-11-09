package importer

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Logger records every parsed value.
//
// The requirement of the importer is that a human must be able to audit
// the conversion before the result is committed: every single record is
// therefore written to the log, together with the table it belongs to.
type Logger interface {
	// Record logs one converted record.
	Record(table string, index int, cells []string)
	// Info logs an informational message.
	Info(format string, args ...any)
	// Problem logs a problem that needs the attention of the user.
	Problem(format string, args ...any)
}

// ConsoleLogger writes the log to a writer.
//
// The default writer is the standard error, so that the log can be piped
// into a file without mixing it with the conversion report.
type ConsoleLogger struct {
	// w is the destination of the log.
	w io.Writer
	// verbose enables the per-record logging.
	verbose bool
	// prefix is prepended to every line.
	prefix string
}

// NewConsoleLogger creates a logger that writes to w.
//
// When verbose is true, every converted record is logged; otherwise only
// the tables and the problems are.
func NewConsoleLogger(w io.Writer, verbose bool) *ConsoleLogger {
	return &ConsoleLogger{w: w, verbose: verbose}
}

// Record implements Logger.
func (l *ConsoleLogger) Record(table string, index int, cells []string) {
	if !l.verbose {
		return
	}
	fmt.Fprintf(l.w, "%s[%s] #%d: %s\n", l.prefix, table, index, strings.Join(cells, " | "))
}

// Info implements Logger.
func (l *ConsoleLogger) Info(format string, args ...any) {
	fmt.Fprintf(l.w, "%s%s\n", l.prefix, fmt.Sprintf(format, args...))
}

// Problem implements Logger.
func (l *ConsoleLogger) Problem(format string, args ...any) {
	fmt.Fprintf(l.w, "%sPROBLEM: %s\n", l.prefix, fmt.Sprintf(format, args...))
}

// FileLogger writes the detailed log into a file and the summary to the
// console.
//
// The detailed log is what a human reads to audit the conversion.
type FileLogger struct {
	// console is the summary logger.
	console *ConsoleLogger
	// detail is the file that holds the per-record log.
	detail *os.File
	// verbose enables the per-record logging.
	verbose bool
}

// NewFileLogger creates a logger that writes the details into a file.
func NewFileLogger(path string, console *ConsoleLogger, verbose bool) (*FileLogger, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &FileLogger{console: console, detail: f, verbose: verbose}, nil
}

// Record implements Logger.
func (l *FileLogger) Record(table string, index int, cells []string) {
	if !l.verbose {
		return
	}
	fmt.Fprintf(l.detail, "%s[%s] #%d: %s\n", "", table, index, strings.Join(cells, " | "))
}

// Info implements Logger.
func (l *FileLogger) Info(format string, args ...any) {
	fmt.Fprintf(l.detail, "INFO: %s\n", fmt.Sprintf(format, args...))
	l.console.Info(format, args...)
}

// Problem implements Logger.
func (l *FileLogger) Problem(format string, args ...any) {
	fmt.Fprintf(l.detail, "PROBLEM: %s\n", fmt.Sprintf(format, args...))
	l.console.Problem(format, args...)
}

// Close closes the detail log file.
func (l *FileLogger) Close() error {
	if l.detail == nil {
		return nil
	}
	return l.detail.Close()
}
