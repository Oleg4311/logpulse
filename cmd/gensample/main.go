// Command gensample writes a deterministic demo nginx access log with a
// traffic spike and a burst of 5xx errors, for trying out logpulse.
package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"time"
)

func main() {
	r := rand.New(rand.NewSource(42))
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	start := time.Date(2026, 10, 10, 13, 0, 0, 0, time.UTC)
	paths := []struct {
		p    string
		w    int
		base float64
	}{
		{"/", 30, 0.02}, {"/api/products", 25, 0.06}, {"/api/products/%d", 20, 0.05},
		{"/api/cart", 10, 0.09}, {"/api/checkout", 5, 0.35}, {"/static/app.js", 8, 0.01}, {"/login", 2, 0.12},
	}
	total := 0
	for _, p := range paths {
		total += p.w
	}
	for sec := 0; sec < 300; sec++ {
		n := 3 + r.Intn(4)
		burst := sec >= 180 && sec < 185
		if burst {
			n = 60
		}
		for i := 0; i < n; i++ {
			x, pick := r.Intn(total), paths[0]
			for _, p := range paths {
				if x < p.w {
					pick = p
					break
				}
				x -= p.w
			}
			path := pick.p
			if path == "/api/products/%d" {
				path = fmt.Sprintf(path, 1+r.Intn(5000))
			}
			status := 200
			switch q := r.Float64(); {
			case burst && pick.p == "/api/checkout" || burst && q < 0.55:
				status = 503
			case q < 0.03:
				status = 404
			case q < 0.04:
				status = 302
			}
			lat := pick.base * (0.5 + r.ExpFloat64())
			if status == 503 {
				lat += 1.5
			}
			t := start.Add(time.Duration(sec) * time.Second)
			method := "GET"
			if pick.p == "/api/checkout" || pick.p == "/login" {
				method = "POST"
			}
			fmt.Fprintf(w, "198.51.100.%d - - [%s] \"%s %s HTTP/1.1\" %d %d \"-\" \"Mozilla/5.0\" %.3f\n",
				1+r.Intn(200), t.Format("02/Jan/2006:15:04:05 -0700"), method, path, status, 200+r.Intn(9000), lat)
		}
	}
}
