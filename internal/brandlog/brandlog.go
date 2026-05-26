package brandlog

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type Mode string

const (
	ModeOff    Mode = "off"
	ModeStdout Mode = "stdout"
	ModeDebug  Mode = "debug"
)

type Options struct {
	Mode    Mode
	Version string
	Writer  io.Writer
}

type Event struct {
	Time   time.Time
	Level  string
	Device int
	Node   string
	Name   string
	Fields map[string]string
}

type Logger struct {
	mode    Mode
	version string
	writer  io.Writer
	mu      sync.Mutex
}

func New(options Options) *Logger {
	writer := options.Writer
	if writer == nil {
		writer = os.Stdout
	}
	version := options.Version
	if version == "" {
		version = "dev"
	}
	return &Logger{mode: options.Mode, version: version, writer: writer}
}

func (l *Logger) Event(event Event) {
	if l == nil || l.mode == ModeOff {
		return
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	if event.Level == "" {
		event.Level = "info"
	}

	parts := []string{
		event.Time.UTC().Format(time.RFC3339),
		"level=" + event.Level,
		"component=hft",
		"version=" + l.version,
	}
	if event.Device >= 0 {
		parts = append(parts, fmt.Sprintf("device=%d", event.Device))
	}
	if event.Node != "" {
		parts = append(parts, "node="+quoteValue(event.Node))
	}
	parts = append(parts, "event="+quoteValue(event.Name))

	keys := make([]string, 0, len(event.Fields))
	for key := range event.Fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts = append(parts, key+"="+quoteValue(Redact(event.Fields[key])))
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintln(l.writer, strings.Join(parts, " "))
}

var keyLikePattern = regexp.MustCompile(`\b(?:sk_|prl1p)[A-Za-z0-9]{8,}\b`)

func Redact(value string) string {
	return keyLikePattern.ReplaceAllStringFunc(value, func(match string) string {
		if len(match) <= 10 {
			return "<redacted>"
		}
		return match[:5] + "..." + match[len(match)-4:]
	})
}

func quoteValue(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	if strings.ContainsAny(value, " \t") {
		return fmt.Sprintf("%q", value)
	}
	return value
}
