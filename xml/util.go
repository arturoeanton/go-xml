package xml

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// ============================================================================
// 1. CHARSET & SANITIZATION
// ============================================================================

// windows1252Table mapea cada byte (0-255) a su runa UTF-8 correspondiente.
var windows1252Table = [256]rune{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
	0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F,
	0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2A, 0x2B, 0x2C, 0x2D, 0x2E, 0x2F,
	0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3A, 0x3B, 0x3C, 0x3D, 0x3E, 0x3F,
	0x40, 0x41, 0x42, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4A, 0x4B, 0x4C, 0x4D, 0x4E, 0x4F,
	0x50, 0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5A, 0x5B, 0x5C, 0x5D, 0x5E, 0x5F,
	0x60, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6A, 0x6B, 0x6C, 0x6D, 0x6E, 0x6F,
	0x70, 0x71, 0x72, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79, 0x7A, 0x7B, 0x7C, 0x7D, 0x7E, 0x7F,
	// 0x80 - 0xFF (Windows-1252 extension & ISO-8859-1)
	0x20AC, 0x0081, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, 0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0x008D, 0x017D, 0x008F,
	0x0090, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, 0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0x009D, 0x017E, 0x0178,
	0x00A0, 0x00A1, 0x00A2, 0x00A3, 0x00A4, 0x00A5, 0x00A6, 0x00A7, 0x00A8, 0x00A9, 0x00AA, 0x00AB, 0x00AC, 0x00AD, 0x00AE, 0x00AF,
	0x00B0, 0x00B1, 0x00B2, 0x00B3, 0x00B4, 0x00B5, 0x00B6, 0x00B7, 0x00B8, 0x00B9, 0x00BA, 0x00BB, 0x00BC, 0x00BD, 0x00BE, 0x00BF,
	0x00C0, 0x00C1, 0x00C2, 0x00C3, 0x00C4, 0x00C5, 0x00C6, 0x00C7, 0x00C8, 0x00C9, 0x00CA, 0x00CB, 0x00CC, 0x00CD, 0x00CE, 0x00CF,
	0x00D0, 0x00D1, 0x00D2, 0x00D3, 0x00D4, 0x00D5, 0x00D6, 0x00D7, 0x00D8, 0x00D9, 0x00DA, 0x00DB, 0x00DC, 0x00DD, 0x00DE, 0x00DF,
	0x00E0, 0x00E1, 0x00E2, 0x00E3, 0x00E4, 0x00E5, 0x00E6, 0x00E7, 0x00E8, 0x00E9, 0x00EA, 0x00EB, 0x00EC, 0x00ED, 0x00EE, 0x00EF,
	0x00F0, 0x00F1, 0x00F2, 0x00F3, 0x00F4, 0x00F5, 0x00F6, 0x00F7, 0x00F8, 0x00F9, 0x00FA, 0x00FB, 0x00FC, 0x00FD, 0x00FE, 0x00FF,
}

// latin1Reader implementa io.Reader para decodificar ISO-8859-1 en vuelo.
type latin1Reader struct {
	r io.Reader
}

func (l *latin1Reader) Read(p []byte) (n int, err error) {
	maxRead := len(p) / 4
	if maxRead == 0 && len(p) > 0 {
		maxRead = 1
	}
	buf := make([]byte, maxRead)
	nRead, errRead := l.r.Read(buf)

	bytesWritten := 0
	for i := 0; i < nRead; i++ {
		r := windows1252Table[buf[i]]
		if bytesWritten+utf8.RuneLen(r) > len(p) {
			break
		}
		w := utf8.EncodeRune(p[bytesWritten:], r)
		bytesWritten += w
	}
	return bytesWritten, errRead
}

// charsetReader inyecta soporte para ISO-8859-1 en el decodificador XML.
func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	charset = strings.ToLower(charset)
	switch charset {
	case "iso-8859-1", "latin1", "windows-1252", "cp1252":
		return &latin1Reader{r: input}, nil
	case "utf-8", "utf8":
		return input, nil
	default:
		return nil, fmt.Errorf("unsupported charset: %s", charset)
	}
}

// sanitizeSoup protege tags peligrosos en modo "Soup" (HTML sucio).
func sanitizeSoup(r io.Reader) io.Reader {
	data, err := io.ReadAll(r)
	if err != nil {
		return r
	}
	// Tags que contienen "raw text" en HTML y rompen parsers XML estrictos
	rawTags := []string{"script", "style", "code", "pre", "textarea"}

	for _, tag := range rawTags {
		pattern := fmt.Sprintf(`(?is)(<%s(?:>|\s[^>]*>))(.*?)(</%s>)`, tag, tag)
		re := regexp.MustCompile(pattern)
		data = re.ReplaceAllFunc(data, func(match []byte) []byte {
			parts := re.FindSubmatch(match)
			if len(parts) < 4 {
				return match
			}
			// Envolvemos el contenido en CDATA para protegerlo
			openTag := parts[1]
			content := parts[2]
			closeTag := parts[3]

			// Escape de CDATA anidado si existiese
			escapedContent := bytes.ReplaceAll(content, []byte("]]>"), []byte("]]]]><![CDATA[>"))

			var buf bytes.Buffer
			buf.Write(openTag)
			buf.WriteString("<![CDATA[")
			buf.Write(escapedContent)
			buf.WriteString("]]>")
			buf.Write(closeTag)
			return buf.Bytes()
		})
	}
	return bytes.NewReader(data)
}

// ============================================================================
// 2. TYPE COERCION (SAFE GETTERS)
// ============================================================================

// AsString fuerza la conversión a string.
func AsString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case fmt.Stringer:
		return t.String()
	case error:
		return t.Error()
	}
	if reflect.TypeOf(v).Kind() == reflect.Map || reflect.TypeOf(v).Kind() == reflect.Slice {
		b, _ := json.Marshal(v)
		return string(b)
	}
	return fmt.Sprintf("%v", v)
}

// AsInt fuerza la conversión a int (0 si falla).
func AsInt(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case float64:
		return int(t)
	case bool:
		if t {
			return 1
		}
		return 0
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(t))
		return i
	}
	return 0
}

// AsFloat fuerza la conversión a float64.
func AsFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f
	}
	return 0.0
}

// AsBool fuerza la conversión a bool.
func AsBool(v any) bool {
	s := strings.ToLower(fmt.Sprintf("%v", v))
	return s == "true" || s == "1" || s == "yes" || s == "on" || s == "ok"
}

// AsSlice garantiza retornar un []any.
func AsSlice(v any) []any {
	if v == nil {
		return []any{}
	}
	if list, ok := v.([]any); ok {
		return list
	}
	return []any{v}
}

// AsTime intenta parsear una fecha con varios layouts comunes.
func AsTime(v any, layouts ...string) (time.Time, error) {
	s := AsString(v)
	if len(layouts) == 0 {
		layouts = []string{
			time.RFC3339,
			"2006-01-02",
			"2006-01-02 15:04:05",
			time.RFC1123,
		}
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse time: %s", s)
}

// ============================================================================
// 3. GLOBAL HELPERS
// ============================================================================

// ToJSON converts the XML map or OrderedMap into a JSON string.
// This helper is particularly useful for debugging purposes or for preparing API responses.
func ToJSON(data any) (string, error) {

	if om, ok := data.(*OrderedMap); ok {
		return om.ToJSON()
	}
	if r, ok := data.(io.Reader); ok {
		b, err := ReaderToJSON(r)
		return string(b), err
	}

	b, err := json.Marshal(data)
	return string(b), err
}

// Text extracts ALL text content recursively from a node and its children.
// Equivalent to jQuery's .text().
func Text(data any) string {
	var sb strings.Builder
	textRecursive(data, &sb)
	return strings.TrimSpace(sb.String())
}

func textRecursive(data any, sb *strings.Builder) {
	if data == nil {
		return
	}
	switch v := data.(type) {
	case string:
		sb.WriteString(v)
	case int, float64, bool:
		sb.WriteString(fmt.Sprintf("%v", v))
	case *OrderedMap:
		if seqAny := v.Get("#seq"); seqAny != nil {
			if seq, ok := seqAny.([]any); ok {
				for _, item := range seq {
					textRecursive(item, sb)
				}
				return
			}
		}
		if t := v.Get("#text"); t != nil {
			sb.WriteString(fmt.Sprintf("%v", t))
		}
		v.ForEach(func(k string, val any) bool {
			if !strings.HasPrefix(k, "@") && k != "#text" && k != "#seq" {
				textRecursive(val, sb)
			}
			return true
		})
	case map[string]any:
		if seq, ok := v["#seq"].([]any); ok {
			for _, item := range seq {
				textRecursive(item, sb)
			}
			return
		}
		if t, ok := v["#text"]; ok {
			sb.WriteString(fmt.Sprintf("%v", t))
		}
		for k, val := range v {
			if !strings.HasPrefix(k, "@") && k != "#text" && k != "#seq" {
				textRecursive(val, sb)
			}
		}
	case []any:
		for _, item := range v {
			textRecursive(item, sb)
		}
	}
}
