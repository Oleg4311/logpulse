// Package render prints a stats.Report as terminal text.
package render

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Oleg4311/logpulse/internal/stats"
)

// Options control the text output.
type Options struct {
	Color bool
}

func (o Options) paint(code, s string) string {
	if !o.Color {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func bar(frac float64, width int) string {
	n := int(frac*float64(width) + 0.5)
	if n > width {
		n = width
	}
	return strings.Repeat("#", n) + strings.Repeat(".", width-n)
}

func human(b int64) string {
	const u = 1024
	if b < u {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(u), 0
	for n := b / u; n >= u; n /= u {
		div *= u
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// Text writes the report to w.
func Text(w io.Writer, r stats.Report, o Options) {
	if r.Total == 0 {
		fmt.Fprintln(w, "No log entries recognized.")
		if r.Skipped > 0 {
			fmt.Fprintf(w, "%d lines skipped (unknown format).\n", r.Skipped)
		}
		return
	}
	fmt.Fprintln(w, o.paint("1", "logpulse report"))
	fmt.Fprintf(w, "  requests   %d   (skipped lines: %d)\n", r.Total, r.Skipped)
	fmt.Fprintf(w, "  period     %s -> %s\n", r.Start.Format("2006-01-02 15:04:05"), r.End.Format("15:04:05"))
	fmt.Fprintf(w, "  rate       avg %.2f req/s, peak %.2f req/s (per %ds bucket)\n", r.AvgRPS, r.PeakRPS, r.BucketSec)
	fmt.Fprintf(w, "  traffic    %s\n", human(r.Bytes))

	fmt.Fprintln(w, "\n"+o.paint("1", "Status codes"))
	classes := make([]string, 0, len(r.StatusClass))
	for c := range r.StatusClass {
		classes = append(classes, c)
	}
	sort.Strings(classes)
	colors := map[string]string{"2xx": "32", "3xx": "36", "4xx": "33", "5xx": "31"}
	for _, c := range classes {
		n := r.StatusClass[c]
		f := float64(n) / float64(r.Total)
		fmt.Fprintf(w, "  %s %s %6.1f%%  %d\n", c, o.paint(colors[c], bar(f, 24)), f*100, n)
	}

	if r.HasLatency {
		fmt.Fprintln(w, "\n"+o.paint("1", "Latency (ms)"))
		fmt.Fprintf(w, "  p50 %.1f   p90 %.1f   p95 %.1f   p99 %.1f   max %.1f\n",
			r.Latency.P50, r.Latency.P90, r.Latency.P95, r.Latency.P99, r.Latency.Max)
	}

	fmt.Fprintln(w, "\n"+o.paint("1", "Top endpoints"))
	for _, p := range r.TopPaths {
		extra := ""
		if p.Errors5 > 0 {
			extra = o.paint("31", fmt.Sprintf("  5xx:%d", p.Errors5))
		}
		fmt.Fprintf(w, "  %7d  %s%s\n", p.Count, p.Endpoint, extra)
	}
	if len(r.SlowPaths) > 0 {
		fmt.Fprintln(w, "\n"+o.paint("1", "Slowest endpoints (p95 ms)"))
		for _, p := range r.SlowPaths {
			fmt.Fprintf(w, "  %9.1f  %s\n", p.P95, p.Endpoint)
		}
	}
	if len(r.Anomalies) > 0 {
		fmt.Fprintln(w, "\n"+o.paint("1;31", "Anomalies"))
		for _, a := range r.Anomalies {
			fmt.Fprintf(w, "  %s  %-6s %s\n", a.Time.Format("15:04:05"), a.Kind, a.Detail)
		}
	}
}
