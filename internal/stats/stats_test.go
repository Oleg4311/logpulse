package stats

import (
	"testing"
	"time"

	"github.com/Oleg4311/logpulse/internal/parse"
)

func TestNormalizePath(t *testing.T) {
	cases := map[string]string{
		"/users/123":              "/users/:id",
		"/users/123/orders/9?x=1": "/users/:id/orders/:id",
		"/static/app.js?v=3":      "/static/app.js",
		"/items/550e8400-e29b-41d4-a716-446655440000": "/items/:id",
		"/v1/health": "/v1/health",
		"/":          "/",
	}
	for in, want := range cases {
		if got := NormalizePath(in); got != want {
			t.Errorf("NormalizePath(%q)=%q want %q", in, got, want)
		}
	}
}

func TestPercentile(t *testing.T) {
	s := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	for p, want := range map[float64]float64{50: 5, 90: 9, 95: 10, 100: 10, 1: 1} {
		if got := Percentile(s, p); got != want {
			t.Errorf("p%v=%v want %v", p, got, want)
		}
	}
	if Percentile(nil, 50) != 0 {
		t.Error("empty must be 0")
	}
}

var base = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func entry(sec int, path string, status int, ms int) parse.Entry {
	return parse.Entry{Time: base.Add(time.Duration(sec) * time.Second), Method: "GET", Path: path,
		Status: status, Bytes: 100, Latency: time.Duration(ms) * time.Millisecond, HasLatency: true}
}

func TestReportBasics(t *testing.T) {
	a := New()
	for i := 0; i < 10; i++ {
		a.Add(entry(i, "/users/"+string(rune('1'+i%9)), 200, 10*(i+1)))
	}
	a.Add(entry(10, "/boom", 500, 1000))
	r := a.Report(5)
	if r.Total != 11 || r.StatusClass["2xx"] != 10 || r.StatusClass["5xx"] != 1 {
		t.Fatalf("%+v", r)
	}
	if r.TopPaths[0].Endpoint != "GET /users/:id" || r.TopPaths[0].Count != 10 {
		t.Fatalf("top: %+v", r.TopPaths)
	}
	if r.Latency.Max != 1000 || r.Latency.P50 != 60 {
		t.Fatalf("latency %+v", r.Latency)
	}
	if r.Bytes != 1100 {
		t.Fatalf("bytes %d", r.Bytes)
	}
}

func TestSpikeAndErrorDetection(t *testing.T) {
	a := New()
	for s := 0; s < 60; s++ { // calm: 1 req/s
		a.Add(entry(s, "/a", 200, 5))
	}
	for i := 0; i < 40; i++ { // burst of failures at second 30
		a.Add(entry(30, "/pay", 503, 800))
	}
	r := a.Report(5)
	kinds := map[string]bool{}
	for _, an := range r.Anomalies {
		if !an.Time.Equal(base.Add(30 * time.Second)) {
			t.Errorf("anomaly at wrong bucket: %+v", an)
		}
		kinds[an.Kind] = true
	}
	if !kinds["spike"] || !kinds["errors"] {
		t.Fatalf("expected spike+errors, got %+v", r.Anomalies)
	}
	if r.PeakRPS != 41 {
		t.Fatalf("peak %v", r.PeakRPS)
	}
}

func TestQuietLogHasNoAnomalies(t *testing.T) {
	a := New()
	for s := 0; s < 100; s++ {
		a.Add(entry(s, "/a", 200, 5))
	}
	if r := a.Report(3); len(r.Anomalies) != 0 {
		t.Fatalf("unexpected %+v", r.Anomalies)
	}
}

func TestEmpty(t *testing.T) {
	r := New().Report(3)
	if r.Total != 0 || r.TopPaths == nil || r.Anomalies == nil {
		t.Fatalf("%+v", r)
	}
}
