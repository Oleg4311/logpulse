// Package stats aggregates parsed entries into a report.
package stats

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/Oleg4311/logpulse/internal/parse"
)

var idRe = regexp.MustCompile(`/(\d+|[0-9a-fA-F]{8}-[0-9a-fA-F-]{27}|[0-9a-fA-F]{24,})(/|$)`)

// NormalizePath strips the query string and replaces numeric/UUID segments
// with :id so /users/1 and /users/2 are grouped together.
func NormalizePath(p string) string {
	for i := 0; i < len(p); i++ {
		if p[i] == '?' || p[i] == '#' {
			p = p[:i]
			break
		}
	}
	for { // repeat: regexp matches overlap on adjacent segments
		n := idRe.ReplaceAllString(p, "/:id$2")
		if n == p {
			return p
		}
		p = n
	}
}

type pathAgg struct {
	count, errs int
	lat         []float64
}

// Aggregator collects entries. It is not safe for concurrent use.
type Aggregator struct {
	entries []parse.Entry
	paths   map[string]*pathAgg
	lat     []float64
	status  map[int]int
	bytes   int64
	Skipped int
	first   time.Time
	last    time.Time
}

// New returns an empty Aggregator.
func New() *Aggregator {
	return &Aggregator{paths: map[string]*pathAgg{}, status: map[int]int{}}
}

// Add records one entry.
func (a *Aggregator) Add(e parse.Entry) {
	e.Path = NormalizePath(e.Path)
	a.entries = append(a.entries, e)
	if a.first.IsZero() || e.Time.Before(a.first) {
		a.first = e.Time
	}
	if e.Time.After(a.last) {
		a.last = e.Time
	}
	a.status[e.Status]++
	a.bytes += e.Bytes
	key := e.Method + " " + e.Path
	p := a.paths[key]
	if p == nil {
		p = &pathAgg{}
		a.paths[key] = p
	}
	p.count++
	if e.Status >= 500 {
		p.errs++
	}
	if e.HasLatency {
		ms := float64(e.Latency) / float64(time.Millisecond)
		a.lat = append(a.lat, ms)
		p.lat = append(p.lat, ms)
	}
}

// Latency holds percentile values in milliseconds.
type Latency struct {
	P50 float64 `json:"p50"`
	P90 float64 `json:"p90"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
	Max float64 `json:"max"`
}

// PathStat is a per-endpoint summary.
type PathStat struct {
	Endpoint string  `json:"endpoint"`
	Count    int     `json:"count"`
	Errors5  int     `json:"errors5xx"`
	P95      float64 `json:"p95ms"`
}

// Anomaly describes an unusual time bucket.
type Anomaly struct {
	Time   time.Time `json:"time"`
	Kind   string    `json:"kind"` // "spike" or "errors"
	Detail string    `json:"detail"`
}

// Report is the final summary.
type Report struct {
	Total       int            `json:"total"`
	Skipped     int            `json:"skipped"`
	Start       time.Time      `json:"start"`
	End         time.Time      `json:"end"`
	AvgRPS      float64        `json:"avgRps"`
	PeakRPS     float64        `json:"peakRps"`
	BucketSec   int            `json:"bucketSeconds"`
	Bytes       int64          `json:"bytes"`
	StatusClass map[string]int `json:"statusClasses"`
	HasLatency  bool           `json:"hasLatency"`
	Latency     Latency        `json:"latency"`
	TopPaths    []PathStat     `json:"topPaths"`
	SlowPaths   []PathStat     `json:"slowPaths"`
	Anomalies   []Anomaly      `json:"anomalies"`
}

// Percentile returns the nearest-rank percentile (0<p<=100) of sorted values.
func Percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(math.Ceil(p / 100 * float64(len(sorted))))
	if rank < 1 {
		rank = 1
	}
	return sorted[rank-1]
}

// Report builds the summary, listing at most top endpoints.
func (a *Aggregator) Report(top int) Report {
	r := Report{Total: len(a.entries), Skipped: a.Skipped, Start: a.first, End: a.last,
		Bytes: a.bytes, StatusClass: map[string]int{}, TopPaths: []PathStat{}, SlowPaths: []PathStat{}, Anomalies: []Anomaly{}}
	if r.Total == 0 {
		return r
	}
	for code, n := range a.status {
		r.StatusClass[strconv.Itoa(code/100)+"xx"] += n
	}
	if len(a.lat) > 0 {
		r.HasLatency = true
		s := append([]float64(nil), a.lat...)
		sort.Float64s(s)
		r.Latency = Latency{Percentile(s, 50), Percentile(s, 90), Percentile(s, 95), Percentile(s, 99), s[len(s)-1]}
	}
	span := a.last.Sub(a.first)
	r.AvgRPS = float64(r.Total) / math.Max(span.Seconds(), 1)

	var all, slow []PathStat
	for k, p := range a.paths {
		ps := PathStat{Endpoint: k, Count: p.count, Errors5: p.errs}
		if len(p.lat) > 0 {
			s := append([]float64(nil), p.lat...)
			sort.Float64s(s)
			ps.P95 = Percentile(s, 95)
			if len(s) >= 3 {
				slow = append(slow, ps)
			}
		}
		all = append(all, ps)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Count != all[j].Count {
			return all[i].Count > all[j].Count
		}
		return all[i].Endpoint < all[j].Endpoint
	})
	sort.Slice(slow, func(i, j int) bool {
		if slow[i].P95 != slow[j].P95 {
			return slow[i].P95 > slow[j].P95
		}
		return slow[i].Endpoint < slow[j].Endpoint
	})
	if top > 0 && len(all) > top {
		all = all[:top]
	}
	if top > 0 && len(slow) > top {
		slow = slow[:top]
	}
	r.TopPaths, r.SlowPaths = all, slow
	r.PeakRPS, r.BucketSec, r.Anomalies = a.buckets(span)
	return r
}

func (a *Aggregator) buckets(span time.Duration) (peak float64, bucketSec int, out []Anomaly) {
	out = []Anomaly{}
	bucketSec = 1
	switch {
	case span > 12*time.Hour:
		bucketSec = 3600
	case span > 10*time.Minute:
		bucketSec = 60
	}
	size := time.Duration(bucketSec) * time.Second
	start := a.first.Truncate(size)
	n := int(a.last.Sub(start)/size) + 1
	if n > 1_000_000 {
		return 0, bucketSec, out
	}
	cnt := make([]int, n)
	errs := make([]int, n)
	for _, e := range a.entries {
		i := int(e.Time.Sub(start) / size)
		cnt[i]++
		if e.Status >= 500 {
			errs[i]++
		}
	}
	var sum, sq float64
	maxC := 0
	for _, c := range cnt {
		sum += float64(c)
		sq += float64(c) * float64(c)
		if c > maxC {
			maxC = c
		}
	}
	peak = float64(maxC) / float64(bucketSec)
	mean := sum / float64(n)
	std := math.Sqrt(math.Max(sq/float64(n)-mean*mean, 0))
	for i := range cnt {
		t := start.Add(time.Duration(i) * size)
		if cnt[i] >= 5 && float64(cnt[i]) > mean+3*std {
			out = append(out, Anomaly{t, "spike", "traffic spike: " + strconv.Itoa(cnt[i]) + " requests (avg " + strconv.FormatFloat(mean, 'f', 1, 64) + ")"})
		}
		if cnt[i] >= 5 && float64(errs[i])/float64(cnt[i]) >= 0.5 {
			out = append(out, Anomaly{t, "errors", strconv.Itoa(errs[i]) + " of " + strconv.Itoa(cnt[i]) + " requests are 5xx"})
		}
	}
	if len(out) > 10 {
		out = out[:10]
	}
	return
}
