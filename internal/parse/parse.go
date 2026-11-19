// Package parse turns raw log lines (nginx/Apache access logs or JSON lines)
// into normalized entries.
package parse

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Entry is one normalized request record.
type Entry struct {
	Time       time.Time
	Method     string
	Path       string
	Status     int
	Bytes      int64
	Latency    time.Duration
	HasLatency bool
}

// Format selects the line format.
type Format string

// Supported formats.
const (
	Auto  Format = "auto"
	Nginx Format = "nginx"
	JSON  Format = "json"
)

var nginxRe = regexp.MustCompile(`^(\S+) \S+ \S+ \[([^\]]+)\] "([^"]*)" (\d{3}) (\d+|-)(?: "[^"]*" "[^"]*"(?: (?:rt=)?([\d.]+))?)?`)

// ErrBadLine is returned for lines that do not match the expected format.
var ErrBadLine = errors.New("unrecognized log line")

// Line parses a single line in the given format.
func Line(line string, f Format) (Entry, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Entry{}, ErrBadLine
	}
	switch f {
	case JSON:
		return jsonLine(line)
	case Nginx:
		return nginxLine(line)
	default:
		if line[0] == '{' {
			return jsonLine(line)
		}
		return nginxLine(line)
	}
}

func nginxLine(line string) (Entry, error) {
	m := nginxRe.FindStringSubmatch(line)
	if m == nil {
		return Entry{}, ErrBadLine
	}
	t, err := time.Parse("02/Jan/2006:15:04:05 -0700", m[2])
	if err != nil {
		return Entry{}, ErrBadLine
	}
	parts := strings.Fields(m[3])
	if len(parts) < 2 {
		return Entry{}, ErrBadLine
	}
	status, _ := strconv.Atoi(m[4])
	e := Entry{Time: t, Method: parts[0], Path: parts[1], Status: status}
	if m[5] != "-" {
		e.Bytes, _ = strconv.ParseInt(m[5], 10, 64)
	}
	if m[6] != "" {
		if sec, err := strconv.ParseFloat(m[6], 64); err == nil {
			e.Latency = time.Duration(sec * float64(time.Second))
			e.HasLatency = true
		}
	}
	return e, nil
}

func jsonLine(line string) (Entry, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		return Entry{}, ErrBadLine
	}
	e := Entry{}
	ok := false
	for _, k := range []string{"time", "ts", "timestamp", "@timestamp", "t"} {
		if v, found := m[k]; found {
			if e.Time, ok = toTime(v); ok {
				break
			}
		}
	}
	if !ok {
		return Entry{}, ErrBadLine
	}
	e.Method = str(m, "method", "verb")
	e.Path = str(m, "path", "uri", "url", "request_uri")
	if e.Path == "" {
		return Entry{}, ErrBadLine
	}
	if e.Method == "" {
		e.Method = "-"
	}
	f, found := num(m, "status", "code")
	if !found {
		return Entry{}, ErrBadLine
	}
	e.Status = int(f)
	if f, found := num(m, "bytes", "size", "body_bytes_sent"); found {
		e.Bytes = int64(f)
	}
	if f, found := num(m, "duration_ms", "latency_ms"); found {
		e.Latency, e.HasLatency = time.Duration(f*float64(time.Millisecond)), true
	} else if f, found := num(m, "duration", "latency", "request_time", "elapsed"); found {
		e.Latency, e.HasLatency = time.Duration(f*float64(time.Second)), true
	}
	return e, nil
}

func str(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func num(m map[string]any, keys ...string) (float64, bool) {
	for _, k := range keys {
		switch v := m[k].(type) {
		case float64:
			return v, true
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f, true
			}
		}
	}
	return 0, false
}

func toTime(v any) (time.Time, bool) {
	switch x := v.(type) {
	case string:
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "02/Jan/2006:15:04:05 -0700"} {
			if t, err := time.Parse(layout, x); err == nil {
				return t, true
			}
		}
	case float64:
		if x > 1e11 { // milliseconds
			return time.UnixMilli(int64(x)).UTC(), true
		}
		return time.Unix(int64(x), int64((x-float64(int64(x)))*1e9)).UTC(), true
	}
	return time.Time{}, false
}
