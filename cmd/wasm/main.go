//go:build js && wasm

// Command wasm exposes the analyzer to the browser: logpulseAnalyze(text, top) -> JSON string.
package main

import (
	"bufio"
	"encoding/json"
	"strings"
	"syscall/js"

	"github.com/Oleg4311/logpulse/internal/parse"
	"github.com/Oleg4311/logpulse/internal/stats"
)

func analyze(_ js.Value, args []js.Value) any {
	text := args[0].String()
	top := 10
	if len(args) > 1 {
		top = args[1].Int()
	}
	agg := stats.New()
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		e, err := parse.Line(sc.Text(), parse.Auto)
		if err != nil {
			agg.Skipped++
			continue
		}
		agg.Add(e)
	}
	b, _ := json.Marshal(agg.Report(top))
	return string(b)
}

func main() {
	js.Global().Set("logpulseAnalyze", js.FuncOf(analyze))
	select {}
}
