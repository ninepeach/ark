package logger

import (
	"fmt"
	"log"
	"os"
	"sync"
)

// Logger represents the server logger (stdout or file-based).
type Logger struct {
	sync.Mutex
	logger     *log.Logger
	debug      bool
	trace      bool
	infoLabel  string
	warnLabel  string
	errorLabel string
	fatalLabel string
	debugLabel string
	traceLabel string
	fl         *FileLogger // non-nil only when file logging is enabled
}

type LogOption interface{ isLoggerOption() }

// LogUTC controls whether timestamps should be UTC.
type LogUTC bool

func (l LogUTC) isLoggerOption() {}

func logFlags(useTime bool, opts ...LogOption) int {
	flags := 0
	if useTime {
		flags = log.LstdFlags | log.Lmicroseconds
	}
	for _, opt := range opts {
		if utc, ok := opt.(LogUTC); ok && useTime && bool(utc) {
			flags |= log.LUTC
		}
	}
	return flags
}

// ----------------------------------------------------------------------
// Standard output logger
// ----------------------------------------------------------------------

func NewStdLogger(useTime, debug, trace, colors, pid bool, opts ...LogOption) *Logger {
	flags := logFlags(useTime, opts...)
	prefix := ""
	if pid {
		prefix = pidPrefix()
	}

	l := &Logger{
		logger: log.New(os.Stderr, prefix, flags),
		debug:  debug,
		trace:  trace,
	}

	if colors {
		setColoredLabelFormats(l)
	} else {
		setPlainLabelFormats(l)
	}
	return l
}

// ----------------------------------------------------------------------
// File logger
// ----------------------------------------------------------------------

func NewFileLogger(filename string, useTime, debug, trace, pid bool, opts ...LogOption) (*Logger, error) {
	flags := logFlags(useTime, opts...)
	prefix := ""
	if pid {
		prefix = pidPrefix()
	}

	fl, err := newFileLogger(filename, prefix, useTime)
	if err != nil {
		return nil, fmt.Errorf("unable to create file logger: %w", err)
	}

	l := &Logger{
		logger: log.New(fl, prefix, flags),
		debug:  debug,
		trace:  trace,
		fl:     fl,
	}

	// FileLogger needs back-reference for internal logging; safe to set here
	fl.Lock()
	fl.logger = l
	fl.Unlock()

	setPlainLabelFormats(l)
	return l, nil
}

// ----------------------------------------------------------------------
// File-logger only features
// ----------------------------------------------------------------------

func (l *Logger) SetSizeLimit(limit int64) error {
	l.Lock()
	fl := l.fl
	l.Unlock()

	if fl == nil {
		return fmt.Errorf("SetSizeLimit requires file logger")
	}
	fl.setLimit(limit)
	return nil
}

func (l *Logger) SetMaxNumFiles(max int) error {
	l.Lock()
	fl := l.fl
	l.Unlock()

	if fl == nil {
		return fmt.Errorf("SetMaxNumFiles requires file logger")
	}
	fl.setMaxNumFiles(max)
	return nil
}

// ----------------------------------------------------------------------
// Lifecycle
// ----------------------------------------------------------------------

func (l *Logger) Close() error {
	if l.fl != nil {
		return l.fl.close()
	}
	return nil
}

// ----------------------------------------------------------------------
// Log label setup
// ----------------------------------------------------------------------

func pidPrefix() string {
	return fmt.Sprintf("[%d] ", os.Getpid())
}

func setPlainLabelFormats(l *Logger) {
	l.infoLabel = "[INF] "
	l.debugLabel = "[DBG] "
	l.warnLabel = "[WRN] "
	l.errorLabel = "[ERR] "
	l.fatalLabel = "[FTL] "
	l.traceLabel = "[TRC] "
}

func setColoredLabelFormats(l *Logger) {
	c := func(code, label string) string {
		return fmt.Sprintf("[\x1b[%sm%s\x1b[0m] ", code, label)
	}

	l.infoLabel = c("32", "INF")
	l.debugLabel = c("36", "DBG")
	l.warnLabel = c("0;93", "WRN")
	l.errorLabel = c("31", "ERR")
	l.fatalLabel = c("31", "FTL")
	l.traceLabel = c("33", "TRC")
}

// ----------------------------------------------------------------------
// Logging API
// ----------------------------------------------------------------------

func (l *Logger) Noticef(format string, v ...any) {
	l.logger.Printf(l.infoLabel+format, v...)
}

func (l *Logger) Warnf(format string, v ...any) {
	l.logger.Printf(l.warnLabel+format, v...)
}

func (l *Logger) Errorf(format string, v ...any) {
	l.logger.Printf(l.errorLabel+format, v...)
}

// Fatalf logs a fatal error and terminates the program.
func (l *Logger) Fatalf(format string, v ...any) {
	l.logger.Fatalf(l.fatalLabel+format, v...)
}

func (l *Logger) Debugf(format string, v ...any) {
	if l.debug {
		l.logger.Printf(l.debugLabel+format, v...)
	}
}

func (l *Logger) Tracef(format string, v ...any) {
	if l.trace {
		l.logger.Printf(l.traceLabel+format, v...)
	}
}