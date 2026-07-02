package xml

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureStdout redirects os.Stdout for the duration of fn and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read captured stdout error = %v", err)
	}
	return string(out)
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	return path
}

func TestGetLastSegment(t *testing.T) {
	cases := map[string]string{
		"a/b/c":  "c",
		"single": "single",
		"a/b/":   "",
	}
	for path, want := range cases {
		if got := getLastSegment(path); got != want {
			t.Errorf("getLastSegment(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestInferCLIValue(t *testing.T) {
	if got := inferCLIValue("true"); got != true {
		t.Errorf("inferCLIValue(true) = %v, want true", got)
	}
	if got := inferCLIValue("false"); got != false {
		t.Errorf("inferCLIValue(false) = %v, want false", got)
	}
	if got := inferCLIValue("hello"); got != "hello" {
		t.Errorf("inferCLIValue(hello) = %v, want hello", got)
	}
}

func TestArrayFlags(t *testing.T) {
	var flags arrayFlags
	if err := flags.Set("a=1"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := flags.Set("b=2"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if len(flags) != 2 || flags[0] != "a=1" || flags[1] != "b=2" {
		t.Errorf("arrayFlags after Set = %v", flags)
	}
	if flags.String() == "" {
		t.Error("String() should not be empty")
	}
}

func TestGetInputReader_File(t *testing.T) {
	path := writeTempFile(t, "in.xml", "<root/>")

	r, err := getInputReader([]string{path})
	if err != nil {
		t.Fatalf("getInputReader() error = %v", err)
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(b) != "<root/>" {
		t.Errorf("getInputReader() content = %q", string(b))
	}
}

func TestGetInputReader_MissingFile(t *testing.T) {
	_, err := getInputReader([]string{filepath.Join(t.TempDir(), "does-not-exist.xml")})
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestCliFormat(t *testing.T) {
	path := writeTempFile(t, "in.xml", `<root><a>1</a></root>`)

	out := captureStdout(t, func() {
		CliFormat([]string{path})
	})

	if !strings.Contains(out, "<root>") || !strings.Contains(out, "<a>1</a>") {
		t.Errorf("CliFormat output missing expected content: %q", out)
	}
}

func TestCliToJson(t *testing.T) {
	path := writeTempFile(t, "in.xml", `<root><a>1</a></root>`)

	out := captureStdout(t, func() {
		CliToJson([]string{path})
	})

	if !strings.Contains(out, `"a"`) {
		t.Errorf("CliToJson output missing expected content: %q", out)
	}
}

func TestCliToCsv(t *testing.T) {
	path := writeTempFile(t, "in.xml", `<orders><order><id>1</id></order><order><id>2</id></order></orders>`)

	out := captureStdout(t, func() {
		CliToCsv([]string{path, "--path=orders/order"})
	})

	if !strings.Contains(out, "id") {
		t.Errorf("CliToCsv output missing header: %q", out)
	}
	if !strings.Contains(out, "1") || !strings.Contains(out, "2") {
		t.Errorf("CliToCsv output missing rows: %q", out)
	}
}

func TestCliQuery(t *testing.T) {
	path := writeTempFile(t, "in.xml", `<root><a>1</a></root>`)

	out := captureStdout(t, func() {
		CliQuery([]string{path, "root/a"})
	})

	if !strings.Contains(out, "1") {
		t.Errorf("CliQuery output missing expected content: %q", out)
	}
}

func TestCliSoap(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(`<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><Response>OK</Response></soap:Body></soap:Envelope>`))
	}))
	defer ts.Close()

	cfgPath := writeTempFile(t, "cfg.json", `{
		"endpoint": "`+ts.URL+`",
		"namespace": "http://test.com",
		"action": "Process",
		"payload": {"Id": "1"}
	}`)

	out := captureStdout(t, func() {
		CliSoap([]string{cfgPath})
	})

	if !strings.Contains(out, "Response") {
		t.Errorf("CliSoap output missing expected content: %q", out)
	}
}

func TestCliSoapQuick(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(bodyBytes), "<Id>42</Id>") {
			t.Errorf("request body missing Id: %s", bodyBytes)
		}
		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(`<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><Response>OK</Response></soap:Body></soap:Envelope>`))
	}))
	defer ts.Close()

	out := captureStdout(t, func() {
		CliSoapQuick([]string{
			"--url=" + ts.URL,
			"--action=Process",
			"--ns=http://test.com",
			"--data=Id=42",
		})
	})

	if !strings.Contains(out, "Response") {
		t.Errorf("CliSoapQuick output missing expected content: %q", out)
	}
}
