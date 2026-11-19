package render

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Oleg4311/logpulse/internal/stats"
)

func sample() stats.Report {
	return stats.Report{
		Total: 200, Skipped: 3,
		Start: time.Date(2026, 10, 10, 13, 0, 0, 0, time.UTC), End: time.Date(2026, 10, 10, 13, 4, 59, 0, time.UTC),
		AvgRPS: 0.67, PeakRPS: 40, BucketSec: 1, Bytes: 5 << 20,
		StatusClass: map[string]int{"2xx": 150, "4xx": 30, "5xx": 20},
		HasLatency:  true, Latency: stats.Latency{P50: 12, P90: 80, P95: 120, P99: 900, Max: 1500},
		TopPaths:  []stats.PathStat{{Endpoint: "GET /api/a", Count: 120, Errors5: 20}, {Endpoint: "POST /b", Count: 80}},
		SlowPaths: []stats.PathStat{{Endpoint: "POST /b", Count: 80, P95: 700}},
		Anomalies: []stats.Anomaly{{Time: time.Date(2026, 10, 10, 13, 3, 0, 0, time.UTC), Kind: "spike", Detail: "traffic spike: 60 requests"}},
	}
}

func render(r stats.Report, color bool) string {
	var b bytes.Buffer
	Text(&b, r, Options{Color: color})
	return b.String()
}

func TestTextContainsAllSections(t *testing.T) {
	out := render(sample(), false)
	for _, want := range []string{
		"logpulse report", "requests   200", "skipped lines: 3", "peak 40.00 req/s", "5.0 MiB",
		"Status codes", "2xx", "75.0%", "5xx", "10.0%",
		"Latency (ms)", "p99 900.0", "max 1500.0",
		"Top endpoints", "GET /api/a", "5xx:20", "POST /b",
		"Slowest endpoints", "700.0",
		"Anomalies", "13:03:00", "spike", "traffic spike: 60 requests",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("в отчёте нет %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("без цвета не должно быть ANSI-последовательностей")
	}
}

func TestColorAddsANSI(t *testing.T) {
	if out := render(sample(), true); !strings.Contains(out, "\x1b[31m") || !strings.Contains(out, "\x1b[0m") {
		t.Errorf("с цветом ожидались ANSI-коды: %q", out)
	}
}

func TestSectionsAreOmittedWhenEmpty(t *testing.T) {
	r := sample()
	r.HasLatency, r.SlowPaths, r.Anomalies = false, nil, nil
	out := render(r, false)
	for _, gone := range []string{"Latency (ms)", "Slowest endpoints", "Anomalies"} {
		if strings.Contains(out, gone) {
			t.Errorf("раздел %q должен отсутствовать", gone)
		}
	}
}

func TestEmptyReport(t *testing.T) {
	if out := render(stats.Report{}, false); !strings.Contains(out, "No log entries recognized.") {
		t.Errorf("получено %q", out)
	}
	out := render(stats.Report{Skipped: 7}, false)
	if !strings.Contains(out, "7 lines skipped") {
		t.Errorf("получено %q", out)
	}
}

func TestHumanSizes(t *testing.T) {
	for in, want := range map[int64]string{0: "0 B", 1023: "1023 B", 1024: "1.0 KiB", 1536: "1.5 KiB", 1 << 20: "1.0 MiB", 3 << 30: "3.0 GiB"} {
		if got := human(in); got != want {
			t.Errorf("human(%d) = %q, ожидалось %q", in, got, want)
		}
	}
}
