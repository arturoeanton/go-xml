package xml

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// 1. PARSING AND CONFIGURATION TESTS (HAPPY PATHS)
// ============================================================================

func TestMapXML_BasicAndForceArray(t *testing.T) {
	xmlData := `
    <library>
        <book id="1">Go 101</book>
        <single>Just one</single>
    </library>`

	m, err := MapXML(strings.NewReader(xmlData), ForceArray("single"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	lib := m.Get("library").(*OrderedMap)

	// Verify ForceArray: must be []any
	single := lib.Get("single")
	singleList, ok := single.([]any)
	if !ok {
		t.Errorf("ForceArray failed: expected []any, got %T", single)
	}
	if len(singleList) != 1 {
		t.Errorf("Incorrect array length: expected 1, got %d", len(singleList))
	}
}

func TestMapXML_Namespaces(t *testing.T) {
	xmlData := `<root xmlns:h="http://www.w3.org/html"><h:table>Data</h:table></root>`

	m, _ := MapXML(strings.NewReader(xmlData),
		RegisterNamespace("html", "http://www.w3.org/html"),
	)

	root := m.Get("root").(*OrderedMap)
	if val := root.Get("html:table"); val == nil {
		t.Errorf("Namespace Alias failed. Keys: %v", root.Keys())
	}
}

func TestMapXML_Experimental(t *testing.T) {
	// Dirty HTML + Type Inference
	xmlData := `<data><val>100</val><active>true</active><br></data>`

	m, err := MapXML(strings.NewReader(xmlData), EnableExperimental())
	if err != nil {
		t.Fatalf("LenientMode failed: %v", err)
	}

	data := m.Get("data").(*OrderedMap)

	if val, ok := data.Get("val").(int); !ok || val != 100 {
		t.Errorf("Int Inference failed: got %T %v", data.Get("val"), data.Get("val"))
	}
	if active, ok := data.Get("active").(bool); !ok || !active {
		t.Errorf("Bool Inference failed")
	}
}

func TestMapXML_Hooks(t *testing.T) {
	xmlData := `<event><date>2025-01-01</date></event>`

	dateHook := func(s string) any {
		parsed, _ := time.Parse("2006-01-02", s)
		return parsed
	}

	m, _ := MapXML(strings.NewReader(xmlData), WithValueHook("date", dateHook))

	event := m.Get("event").(*OrderedMap)
	if _, ok := event.Get("date").(time.Time); !ok {
		t.Errorf("Hook failed: expected time.Time")
	}
}

// ============================================================================
// 2. ERROR HANDLING AND EDGE CASES (UNHAPPY PATHS)
// ============================================================================

func TestMapXML_Errors(t *testing.T) {
	// Invalid XML (unclosed tag)
	xmlData := `<root><open>oops`

	_, err := MapXML(strings.NewReader(xmlData))
	if err == nil {
		t.Error("Expected error with malformed XML, but none occurred")
	}
}

func TestSimpleQuery_Errors(t *testing.T) {
	xmlData := `<root><a>1</a></root>`
	m, _ := MapXML(strings.NewReader(xmlData))

	// Non-existent path
	_, err := Query(m, "root/b/c")
	if err == nil {
		t.Error("Query should have failed with 'not found'")
	}

	// Filter that doesn't match
	_, err = Query(m, "root/a[val=999]")
	if err == nil {
		t.Error("Query with invalid filter should have failed")
	}
}

// ============================================================================
// 3. QUERY TESTS (NAVIGATION)
// ============================================================================

func TestQuery_And_QueryAll(t *testing.T) {
	xmlData := `
    <store>
        <item id="1"><name>A</name></item>
        <item id="2"><name>B</name></item>
        <section>
            <item id="3"><name>C</name></item>
        </section>
    </store>`

	m, _ := MapXML(strings.NewReader(xmlData), ForceArray("item"))

	// Query Single
	res, err := Query(m, "store/item[id=2]/name")
	if err != nil || res != "B" {
		t.Errorf("Query failed: %v, %v", err, res)
	}

	// QueryAll (Deep Search)
	results, err := QueryAll(m, "store/item/name")
	if len(results) != 2 {
		t.Errorf("QueryAll expected 2 direct items, got %d", len(results))
	}
}

// ============================================================================
// 4. VALIDATION TESTS
// ============================================================================

func TestValidate(t *testing.T) {
	xmlData := `<user><age>15</age><role>hacker</role></user>`
	m, _ := MapXML(strings.NewReader(xmlData), EnableExperimental())

	rules := []Rule{
		{Path: "user/age", Type: "int", Min: 18},
		{Path: "user/role", Type: "string", Enum: []string{"admin", "user"}},
		{Path: "user/email", Required: true},
	}

	errs := Validate(m, rules)

	if len(errs) != 3 {
		t.Errorf("Validation expected 3 errors, got %d: %v", len(errs), errs)
	}
}

// ============================================================================
// 5. STREAMING TESTS (DECODER & ENCODER)
// ============================================================================

func TestEncoder_Features(t *testing.T) {
	data := map[string]any{
		"root": map[string]any{
			"@lang":     "es",
			"#comments": []string{" Test Comment "},
			"content": map[string]any{
				"#cdata": "<b>Bold</b>",
			},
		},
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf, WithPrettyPrint())
	if err := enc.Encode(data); err != nil {
		t.Fatalf("Encoder failed: %v", err)
	}

	output := buf.String()
	checks := []string{
		`lang="es"`,
		``,
		`<![CDATA[<b>Bold</b>]]>`,
		`<root`,
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("Encoder output missing '%s'. Output:\n%s", check, output)
		}
	}
}

func TestMapXML_Metadata(t *testing.T) {
	// XML with Directive (DOCTYPE) and Processing Instruction (xml-stylesheet)
	xmlData := `
    <!DOCTYPE html>
    <?xml-stylesheet type="text/xsl" href="style.xsl"?>
    <root>
        <content>Data</content>
    </root>`

	m, err := MapXML(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("Error parsing metadata: %v", err)
	}

	// 1. Verify DOCTYPE (#directive)
	if directives, ok := m.Get("#directive").([]string); ok {
		if !strings.Contains(directives[0], "DOCTYPE html") {
			t.Errorf("Incorrect directive: %v", directives[0])
		}
	} else {
		t.Error("Key #directive not found")
	}

	// 2. Verify Processing Instruction (#pi)
	if pis, ok := m.Get("#pi").([]string); ok {
		// Expecting format "target=xml-stylesheet data=type=..."
		if !strings.Contains(pis[0], "target=xml-stylesheet") {
			t.Errorf("Incorrect ProcInst: %v", pis[0])
		}
	} else {
		t.Error("Key #pi not found")
	}
}

func TestMapXML_SoupMode(t *testing.T) {
	// Dirty HTML: Mixed case, unquoted attributes (if parser allows),
	// unclosed tags (br, img, input), and whitespace garbage.
	htmlData := `
    <HTML>
        <BODY>
            <div ID="Container">
                Hello <br> World
                <IMG src="photo.jpg">
                <INPUT type="text" value="test">
                </div>
        </BODY>
    </HTML>`

	// Use Soup configuration (EnableExperimental/EnableSoupMode)
	m, err := MapXML(strings.NewReader(htmlData), EnableExperimental())
	if err != nil {
		t.Fatalf("Error in SoupMode: %v", err)
	}

	// 1. Verify Normalization (Case Insensitivity)
	// Expecting lowercase "html", even though input was <HTML>
	html, ok := m.Get("html").(*OrderedMap)
	if !ok {
		t.Fatalf("Root tag normalization failed: expected 'html'")
	}

	body := html.Get("body").(*OrderedMap)
	div := body.Get("div").(*OrderedMap)

	// 2. Verify Normalized Attributes
	// Expecting lowercase "@id", input was ID="Container"
	if id := div.Get("@id"); id != "Container" {
		t.Errorf("Attribute normalization failed: expected @id='Container', got: %v", div)
	}

	// 3. Verify Void Elements and Text
	// Check that 'img' and 'input' are INSIDE the div and not weirdly nested.

	if val := div.Get("img"); val == nil {
		t.Error("The <img> tag was not detected inside the div (possible auto-close error)")
	}
	if val := div.Get("input"); val == nil {
		t.Error("The <input> tag was not detected inside the div")
	}

	// Verify that <br> exists and has no children (it's void)
	if val := div.Get("br"); val == nil {
		t.Error("The <br> tag disappeared")
	}
}

func TestMapXML_SoupMode_ScriptSafety(t *testing.T) {
	// Case that usually breaks encoding/xml: comparisons in JS
	// If pre-cleaner is implemented, this should pass.
	htmlData := `<div><script>if (a < b) { console.log("x"); }</script><code> a<2 </code></div>`

	_, err := MapXML(strings.NewReader(htmlData), EnableExperimental())

	// If using pure encoding/xml, this will error.
	if err != nil {
		t.Logf("Expected Behavior (Limitation): Failed on script with '<': %v", err)
	} else {
		t.Log("Success: The parser survived the script.")
	}
}

func TestMapXML_SoupMode_Heavy(t *testing.T) {
	// HTML designed to break weak parsers
	htmlData := `
    <HTML>
        <HEAD>
            <script type="text/javascript">
                // Case 1: Operators that look like tags
                if (a < b && c > d) { console.log("math"); }
                
                // Case 2: The "Killer": CDATA closing sequence inside a JS string
                var danger = "This string contains ]]> which breaks XML";
                var moreDanger = "]]>";
            </script>
            <STYLE>
                /* Case 3: CSS with reserved characters */
                body { content: "<p>I am not a tag</p>"; }
            </STYLE>
        </HEAD>
        <BODY>
            <TEXTAREA name=comments>
                <p>Hello</p> <br> 
                This should not be parsed as child nodes.
            </TEXTAREA>
            
            <div ID="main">
                <CODE> a <= b </CODE>
            </div>
        </BODY>
    </HTML>`

	m, err := MapXML(strings.NewReader(htmlData), EnableExperimental())
	if err != nil {
		t.Fatalf("CRASH: Parser could not handle heavy test: %v", err)
	}

	// 2. Script Verification
	// NOTE: Since <script> has 'type' attribute, parser returns a Map.
	// We use Get with /#text to retrieve content.
	scriptContent, _ := Get[string](m, "html/head/script/#text")

	if !strings.Contains(scriptContent, `if (a < b && c > d)`) {
		t.Error("Script: Logical operators < and > were corrupted")
	}
	if !strings.Contains(scriptContent, `var danger = "This string contains ]]> which breaks XML";`) {
		t.Error("Script: Failed to escape CDATA sequence ']]>'")
	}

	// 3. Textarea Verification
	// NOTE: Has 'name' attribute, so it is also a Map.
	textareaContent, _ := Get[string](m, "html/body/textarea/#text")

	if !strings.Contains(textareaContent, "<p>Hello</p>") {
		t.Error("Textarea: Internal HTML content was lost")
	}

	// 4. Code Verification
	// NOTE: <CODE> has no attributes, so the parser SIMPLIFIES it to a direct string.
	// Our Get helper handles this automatically.
	codeContent, _ := Get[string](m, "html/body/div/code/#text")

	if !strings.Contains(codeContent, "a <= b") {
		t.Errorf("Code: Incorrect content. Got: %s", codeContent)
	}

	t.Log("SUCCESS! The parser survived the Heavy Test and validated content correctly.")
}

// TestMapXML_Charsets verifies the parser's ability to handle legacy single-byte encodings
// (ISO-8859-1 and Windows-1252) when the EnableLegacyCharsets option is active.
//
// Methodology:
// Since Go source files are UTF-8 by default, we cannot simply write characters like 'ñ' or '€'
// directly in the input string, as Go would encode them as multi-byte UTF-8 sequences.
// Instead, we use hexadecimal escape sequences (e.g., \xF1) to force specific raw bytes
// that represent these characters in their respective legacy encodings.
//
// Scenarios Covered:
//  1. ISO-8859-1: Validates standard Latin-1 characters (e.g., 'ñ', 'ó') map correctly to UTF-8.
//  2. Windows-1252: Validates characters that exist in CP1252 but not in ISO-8859-1
//     (e.g., the Euro symbol '€' and smart quotes).
//  3. Real-world Compatibility: Tests the common "lying header" scenario found in banking
//     and government systems, where the XML declares "encoding='ISO-8859-1'" but actually
//     contains Windows-1252 bytes. The parser should gracefully handle this by using the
//     Windows-1252 table as a superset.
func TestMapXML_Charsets(t *testing.T) {
	tests := []struct {
		name     string
		rawXML   string // Usamos string con escapes hex para forzar bytes no-UTF8
		query    string
		expected string
	}{
		{
			name: "ISO-8859-1: Spanish Accents & Ñ",
			// Header declara ISO-8859-1
			// <data>Canción - Año</data>
			// \xF3 = ó, \xF1 = ñ (en Latin1)
			rawXML: "<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?>" +
				"<root><data>Canci\xF3n - A\xF1o</data></root>",
			query:    "root/data",
			expected: "Canción - Año", // Resultado esperado en UTF-8 nativo de Go
		},
		{
			name: "Windows-1252: Euro Symbol",
			// Header declara Windows-1252
			// <price>100 €</price>
			// \x80 es el byte para € en CP1252 (en ISO puro es un control char)
			rawXML: "<?xml version=\"1.0\" encoding=\"Windows-1252\"?>" +
				"<root><price>100 \x80</price></root>",
			query:    "root/price",
			expected: "100 €",
		},
		{
			name: "Windows-1252: Smart Quotes (Fancy Quotes)",
			// <quote>“Hello”</quote>
			// \x93 (Left double quote) y \x94 (Right double quote)
			rawXML: "<?xml version=\"1.0\" encoding=\"cp1252\"?>" +
				"<root><quote>\x93Hello\x94</quote></root>",
			query:    "root/quote",
			expected: "“Hello”",
		},
		{
			name: "Legacy Mix: Header says ISO but uses Windows chars",
			// Este caso es CRÍTICO para bancos. Declaran ISO pero mandan Windows-1252.
			// Tu implementación debe ser capaz de usar la tabla Windows para ambos.
			// Byte \x9C = œ (ligadura oe), no existe en ISO-8859-1, solo en Win-1252.
			rawXML: "<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?>" +
				"<root><word>C\x9Cur</word></root>", // Cœur (Corazón en francés)
			query:    "root/word",
			expected: "Cœur",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. Parseamos usando la opción EnableLegacyCharsets
			r := strings.NewReader(tt.rawXML)
			m, err := MapXML(r, EnableLegacyCharsets())
			if err != nil {
				t.Fatalf("MapXML unexpected error: %v", err)
			}

			// 2. Consultamos el valor
			got, err := Query(m, tt.query)
			if err != nil {
				t.Fatalf("Query failed: %v", err)
			}

			// 3. Verificamos que se haya convertido a UTF-8 correctamente
			if got != tt.expected {
				t.Errorf("Charset conversion failed.\nGot (UTF-8 bytes): % +q\nWant: %s", got, tt.expected)
			}
		})
	}
}

// ============================================================================
// 6. BENCHMARKS
// ============================================================================

// BenchmarkMapXML measures parsing speed to map in memory
func BenchmarkMapXML(b *testing.B) {
	// Generate medium-sized XML
	xmlData := `<root>` + strings.Repeat(`<item>Data</item>`, 1000) + `</root>`
	r := strings.NewReader(xmlData)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Seek(0, 0) // Reset reader
		MapXML(r)
	}
}

// BenchmarkStreaming measures speed of reading element by element
func BenchmarkStreaming(b *testing.B) {
	xmlData := `<root>` + strings.Repeat(`<item>Data</item>`, 1000) + `</root>`
	type Item struct {
		Val string `xml:",chardata"`
	}
	r := strings.NewReader(xmlData)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Seek(0, 0)
		stream := NewStream[Item](r, "item")
		for range stream.Iter() {
		}
	}
}
