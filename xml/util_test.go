package xml

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// AsString
// ---------------------------------------------------------------------------

type stringerType struct{}

func (stringerType) String() string { return "stringer-value" }

func TestAsString(t *testing.T) {
	if got := AsString(nil); got != "" {
		t.Errorf("AsString(nil) = %q, want \"\"", got)
	}
	if got := AsString("hello"); got != "hello" {
		t.Errorf("AsString(string) = %q, want hello", got)
	}
	if got := AsString([]byte("bytes")); got != "bytes" {
		t.Errorf("AsString([]byte) = %q, want bytes", got)
	}
	if got := AsString(stringerType{}); got != "stringer-value" {
		t.Errorf("AsString(Stringer) = %q, want stringer-value", got)
	}
	if got := AsString(errors.New("boom")); got != "boom" {
		t.Errorf("AsString(error) = %q, want boom", got)
	}
	if got := AsString(map[string]any{"a": 1}); got != `{"a":1}` {
		t.Errorf("AsString(map) = %q, want {\"a\":1}", got)
	}
	if got := AsString([]any{1, 2}); got != `[1,2]` {
		t.Errorf("AsString(slice) = %q, want [1,2]", got)
	}
	if got := AsString(42); got != "42" {
		t.Errorf("AsString(int) = %q, want 42", got)
	}
}

// ---------------------------------------------------------------------------
// AsInt
// ---------------------------------------------------------------------------

func TestAsInt(t *testing.T) {
	cases := []struct {
		in   any
		want int
	}{
		{5, 5},
		{3.9, 3},
		{true, 1},
		{false, 0},
		{" 42 ", 42},
		{"not-a-number", 0},
		{nil, 0},
	}
	for _, c := range cases {
		if got := AsInt(c.in); got != c.want {
			t.Errorf("AsInt(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// AsFloat
// ---------------------------------------------------------------------------

func TestAsFloat(t *testing.T) {
	cases := []struct {
		in   any
		want float64
	}{
		{3.14, 3.14},
		{5, 5.0},
		{" 2.5 ", 2.5},
		{"bad", 0.0},
		{nil, 0.0},
	}
	for _, c := range cases {
		if got := AsFloat(c.in); got != c.want {
			t.Errorf("AsFloat(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// AsBool
// ---------------------------------------------------------------------------

func TestAsBool(t *testing.T) {
	truthy := []any{"true", "TRUE", "1", "yes", "YES", "on", "ok", true}
	for _, v := range truthy {
		if !AsBool(v) {
			t.Errorf("AsBool(%v) = false, want true", v)
		}
	}
	falsy := []any{"false", "0", "no", "off", "", false, nil}
	for _, v := range falsy {
		if AsBool(v) {
			t.Errorf("AsBool(%v) = true, want false", v)
		}
	}
}

// ---------------------------------------------------------------------------
// AsSlice
// ---------------------------------------------------------------------------

func TestAsSlice(t *testing.T) {
	if got := AsSlice(nil); len(got) != 0 {
		t.Errorf("AsSlice(nil) = %v, want empty", got)
	}
	list := []any{1, 2, 3}
	if got := AsSlice(list); len(got) != 3 {
		t.Errorf("AsSlice([]any) = %v, want passthrough of 3 items", got)
	}
	single := AsSlice("solo")
	if len(single) != 1 || single[0] != "solo" {
		t.Errorf("AsSlice(single) = %v, want [\"solo\"]", single)
	}
}

// ---------------------------------------------------------------------------
// AsTime
// ---------------------------------------------------------------------------

func TestAsTime_DefaultLayouts(t *testing.T) {
	cases := []string{
		"2024-03-05T10:00:00Z", // RFC3339
		"2024-03-05",           // date only
		"2024-03-05 10:00:00",  // date + time
	}
	for _, s := range cases {
		got, err := AsTime(s)
		if err != nil {
			t.Errorf("AsTime(%q) error: %v", s, err)
			continue
		}
		if got.Year() != 2024 || got.Month() != time.March || got.Day() != 5 {
			t.Errorf("AsTime(%q) = %v, wrong date", s, got)
		}
	}
}

func TestAsTime_CustomLayout(t *testing.T) {
	got, err := AsTime("05/03/2024", "02/01/2006")
	if err != nil {
		t.Fatalf("AsTime custom layout error: %v", err)
	}
	if got.Day() != 5 || got.Month() != time.March || got.Year() != 2024 {
		t.Errorf("AsTime custom layout = %v, wrong date", got)
	}
}

func TestAsTime_Unparseable(t *testing.T) {
	if _, err := AsTime("not a date"); err == nil {
		t.Fatal("expected error for unparseable date, got nil")
	}
}

// ---------------------------------------------------------------------------
// Text
// ---------------------------------------------------------------------------

func TestText_OrderedMap(t *testing.T) {
	child := NewMap()
	child.Set("#text", "World")

	root := NewMap()
	root.Set("@attr", "ignored")
	root.Set("#text", "Hello")
	root.Set("Child", child)

	got := Text(root)
	want := "HelloWorld"
	if got != want {
		t.Errorf("Text(OrderedMap) = %q, want %q", got, want)
	}
}

func TestText_Map(t *testing.T) {
	m := map[string]any{
		"@attr":  "ignored",
		"#text":  "Hi",
		"nested": map[string]any{"#text": "There"},
	}
	got := Text(m)
	if got != "HiThere" {
		t.Errorf("Text(map) = %q, want HiThere", got)
	}
}

func TestText_Slice(t *testing.T) {
	got := Text([]any{"a", "b", 3})
	if got != "ab3" {
		t.Errorf("Text(slice) = %q, want ab3", got)
	}
}

func TestText_Sequence(t *testing.T) {
	root := NewMap()
	root.Set("#seq", []any{"one", "two"})
	root.Set("#text", "ignored-because-seq-present")

	got := Text(root)
	if got != "onetwo" {
		t.Errorf("Text(#seq) = %q, want onetwo", got)
	}
}

// ---------------------------------------------------------------------------
// charsetReader / latin1Reader
// ---------------------------------------------------------------------------

func TestCharsetReader_KnownAliases(t *testing.T) {
	for _, name := range []string{"iso-8859-1", "ISO-8859-1", "latin1", "windows-1252", "cp1252"} {
		r, err := charsetReader(name, nil)
		if err != nil {
			t.Errorf("charsetReader(%q) error: %v", name, err)
			continue
		}
		if _, ok := r.(*latin1Reader); !ok {
			t.Errorf("charsetReader(%q) returned %T, want *latin1Reader", name, r)
		}
	}
}

func TestCharsetReader_UTF8Passthrough(t *testing.T) {
	src := strings.NewReader("hello")
	r, err := charsetReader("utf-8", src)
	if err != nil {
		t.Fatalf("charsetReader(utf-8) error: %v", err)
	}
	if r != src {
		t.Error("charsetReader(utf-8) should return the input reader unchanged")
	}
}

func TestCharsetReader_Unsupported(t *testing.T) {
	if _, err := charsetReader("shift-jis", nil); err == nil {
		t.Fatal("expected error for unsupported charset, got nil")
	}
}

func TestLatin1Reader_DecodesToUTF8(t *testing.T) {
	// 0xE9 in ISO-8859-1/Windows-1252 is 'é' (U+00E9).
	src := strings.NewReader(string([]byte{0xE9}))
	r := &latin1Reader{r: src}

	buf := make([]byte, 16)
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("Read error: %v", err)
	}
	got := string(buf[:n])
	if got != "é" {
		t.Errorf("latin1Reader decoded %q, want é", got)
	}
}
