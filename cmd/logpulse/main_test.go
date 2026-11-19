package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Настоящий бинарник собирается один раз и запускается как процесс: проверяются флаги, stdin, коды выхода.
var bin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "logpulse-test")
	if err != nil {
		panic(err)
	}
	bin = filepath.Join(dir, "logpulse")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		panic(string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

const sampleLog = "../../testdata/sample.log"

func run(t *testing.T, stdin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Stdin = strings.NewReader(stdin)
	var o, e bytes.Buffer
	cmd.Stdout, cmd.Stderr = &o, &e
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return o.String(), e.String(), code
}

func TestFileReport(t *testing.T) {
	out, _, code := run(t, "", "-no-color", sampleLog)
	if code != 0 || !strings.Contains(out, "requests   1613") || !strings.Contains(out, "traffic spike") {
		t.Fatalf("код %d, вывод:\n%s", code, out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("-no-color не отключил цвета")
	}
}

func TestJSONOutputIsValid(t *testing.T) {
	out, _, code := run(t, "", "-json", "-top", "3", sampleLog)
	var r struct {
		Total    int `json:"total"`
		TopPaths []struct {
			Endpoint string `json:"endpoint"`
		} `json:"topPaths"`
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil || code != 0 {
		t.Fatalf("код %d: %v\n%s", code, err, out)
	}
	if r.Total != 1613 || len(r.TopPaths) != 3 {
		t.Fatalf("total=%d, топ путей=%d", r.Total, len(r.TopPaths))
	}
}

func TestStdinAndFormats(t *testing.T) {
	nginx := `1.2.3.4 - - [10/Oct/2026:13:55:36 +0000] "GET /a HTTP/1.1" 200 10 "-" "ua"` + "\n"
	out, _, _ := run(t, nginx+"мусор\n", "-no-color")
	if !strings.Contains(out, "requests   1   (skipped lines: 1)") {
		t.Fatalf("stdin: %s", out)
	}
	out, _, _ = run(t, nginx, "-no-color", "-format", "json") // nginx-строка в режиме json пропускается
	if !strings.Contains(out, "No log entries recognized.") {
		t.Fatalf("формат json: %s", out)
	}
	jsonLine := `{"time":"2026-10-10T13:55:36Z","method":"GET","path":"/j","status":200}` + "\n"
	if out, _, _ = run(t, jsonLine, "-no-color"); !strings.Contains(out, "GET /j") {
		t.Fatalf("json-строки: %s", out)
	}
}

func TestMultipleFilesAreMerged(t *testing.T) {
	out, _, _ := run(t, "", "-no-color", sampleLog, sampleLog)
	if !strings.Contains(out, "requests   3226") {
		t.Fatalf("два файла должны суммироваться: %s", out)
	}
}

func TestMissingFileFailsClearly(t *testing.T) {
	_, errOut, code := run(t, "", "no-such-file.log")
	if code != 1 || !strings.Contains(errOut, "logpulse:") {
		t.Fatalf("код %d, stderr %q", code, errOut)
	}
}

func TestVersionFlag(t *testing.T) {
	out, _, code := run(t, "", "-version")
	if code != 0 || !strings.HasPrefix(out, "logpulse ") {
		t.Fatalf("код %d: %q", code, out)
	}
}
