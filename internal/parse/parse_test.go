package parse

import (
	"testing"
	"time"
)

func TestNginxCombined(t *testing.T) {
	line := `203.0.113.5 - - [10/Oct/2026:13:55:36 +0000] "GET /api/users/42?x=1 HTTP/1.1" 200 1532 "-" "curl/8.0"`
	e, err := Line(line, Auto)
	if err != nil {
		t.Fatal(err)
	}
	if e.Method != "GET" || e.Path != "/api/users/42?x=1" || e.Status != 200 || e.Bytes != 1532 || e.HasLatency {
		t.Fatalf("unexpected entry: %+v", e)
	}
	if e.Time.Unix() != time.Date(2026, 10, 10, 13, 55, 36, 0, time.UTC).Unix() {
		t.Fatalf("bad time %v", e.Time)
	}
}

func TestNginxRequestTime(t *testing.T) {
	for _, tail := range []string{` 0.250`, ` rt=0.250`} {
		line := `1.2.3.4 - - [10/Oct/2026:13:55:36 +0000] "POST /login HTTP/1.1" 401 12 "-" "ua"` + tail
		e, err := Line(line, Nginx)
		if err != nil || !e.HasLatency || e.Latency != 250*time.Millisecond {
			t.Fatalf("tail %q: %+v %v", tail, e, err)
		}
	}
}

func TestNginxDashBytes(t *testing.T) {
	e, err := Line(`1.2.3.4 - - [10/Oct/2026:13:55:36 +0000] "GET / HTTP/1.1" 304 - "-" "ua"`, Auto)
	if err != nil || e.Bytes != 0 || e.Status != 304 {
		t.Fatalf("%+v %v", e, err)
	}
}

func TestJSON(t *testing.T) {
	cases := []struct {
		name, line string
		lat        time.Duration
	}{
		{"seconds", `{"time":"2026-10-10T13:55:36Z","method":"GET","path":"/a","status":200,"duration":0.5}`, 500 * time.Millisecond},
		{"ms", `{"ts":1791640536,"verb":"GET","uri":"/a","code":"200","duration_ms":12}`, 12 * time.Millisecond},
		{"epoch ms", `{"@timestamp":1791640536000,"method":"GET","url":"/a","status":200,"request_time":"0.1"}`, 100 * time.Millisecond},
	}
	for _, c := range cases {
		e, err := Line(c.line, Auto)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if e.Path != "/a" || e.Status != 200 || e.Latency != c.lat {
			t.Fatalf("%s: %+v", c.name, e)
		}
	}
}

func TestBadLines(t *testing.T) {
	for _, l := range []string{"", "hello world", `{"path":"/a"}`, `{"time":"x","path":"/a","status":200}`, `{broken`,
		`1.2.3.4 - - [bad date] "GET / HTTP/1.1" 200 1 "-" "ua"`} {
		if _, err := Line(l, Auto); err == nil {
			t.Errorf("expected error for %q", l)
		}
	}
}
