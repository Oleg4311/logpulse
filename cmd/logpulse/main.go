// Command logpulse analyzes web server access logs.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/Oleg4311/logpulse/internal/parse"
	"github.com/Oleg4311/logpulse/internal/render"
	"github.com/Oleg4311/logpulse/internal/stats"
)

var version = "dev"

func main() {
	top := flag.Int("top", 10, "number of endpoints to list")
	format := flag.String("format", "auto", "log format: auto, nginx, json")
	asJSON := flag.Bool("json", false, "print the report as JSON")
	noColor := flag.Bool("no-color", false, "disable ANSI colors")
	live := flag.Bool("live", false, "refresh the report while reading (use with: tail -f access.log | logpulse -live)")
	ver := flag.Bool("version", false, "print version")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: logpulse [flags] [file ...]   (reads stdin when no file or \"-\")")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *ver {
		fmt.Println("logpulse", version)
		return
	}

	var mu sync.Mutex
	agg := stats.New()
	f := parse.Format(*format)
	opts := render.Options{Color: !*noColor && !*asJSON && isTTY(os.Stdout)}

	done := make(chan struct{})
	if *live {
		go func() {
			t := time.NewTicker(500 * time.Millisecond)
			defer t.Stop()
			for {
				select {
				case <-t.C:
					mu.Lock()
					r := agg.Report(*top)
					mu.Unlock()
					fmt.Print("\x1b[2J\x1b[H")
					render.Text(os.Stdout, r, opts)
				case <-done:
					return
				}
			}
		}()
	}

	read := func(rd io.Reader) {
		sc := bufio.NewScanner(rd)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			e, err := parse.Line(sc.Text(), f)
			mu.Lock()
			if err != nil {
				agg.Skipped++
			} else {
				agg.Add(e)
			}
			mu.Unlock()
		}
	}

	files := flag.Args()
	if len(files) == 0 {
		files = []string{"-"}
	}
	for _, name := range files {
		if name == "-" {
			read(os.Stdin)
			continue
		}
		fh, err := os.Open(name)
		if err != nil {
			fmt.Fprintln(os.Stderr, "logpulse:", err)
			os.Exit(1)
		}
		read(fh)
		fh.Close()
	}
	close(done)

	mu.Lock()
	r := agg.Report(*top)
	mu.Unlock()
	if *live {
		fmt.Print("\x1b[2J\x1b[H")
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(r)
		return
	}
	render.Text(os.Stdout, r, opts)
}

func isTTY(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}
