package xml

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// 1. TEST DE PARSING Y CONFIGURACIÓN (HAPPY PATHS)
// ============================================================================

func TestMapXML_BasicAndForceArray(t *testing.T) {
	xmlData := `
	<library>
		<book id="1">Go 101</book>
		<single>Solo uno</single>
	</library>`

	m, err := MapXML(strings.NewReader(xmlData), ForceArray("single"))
	if err != nil {
		t.Fatalf("Error inesperado: %v", err)
	}

	lib := m["library"].(map[string]any)

	// Verificar ForceArray: debe ser []any
	single, ok := lib["single"].([]any)
	if !ok {
		t.Errorf("ForceArray falló: se esperaba []any, se obtuvo %T", lib["single"])
	}
	if len(single) != 1 {
		t.Errorf("Longitud de array incorrecta: se esperaba 1, se obtuvo %d", len(single))
	}
}

func TestMapXML_Namespaces(t *testing.T) {
	xmlData := `<root xmlns:h="http://www.w3.org/html"><h:table>Datos</h:table></root>`

	m, _ := MapXML(strings.NewReader(xmlData),
		RegisterNamespace("html", "http://www.w3.org/html"),
	)

	root := m["root"].(map[string]any)
	if _, exists := root["html:table"]; !exists {
		t.Errorf("Namespace Alias falló. Claves encontradas: %v", root)
	}
}

func TestMapXML_Experimental(t *testing.T) {
	// HTML sucio + Inferencia de tipos
	xmlData := `<data><val>100</val><active>true</active><br></data>`

	m, err := MapXML(strings.NewReader(xmlData), EnableExperimental())
	if err != nil {
		t.Fatalf("LenientMode falló: %v", err)
	}

	data := m["data"].(map[string]any)

	if val, ok := data["val"].(int); !ok || val != 100 {
		t.Errorf("Inferencia Int falló: obtuvo %T %v", data["val"], data["val"])
	}
	if active, ok := data["active"].(bool); !ok || !active {
		t.Errorf("Inferencia Bool falló")
	}
}

func TestMapXML_Hooks(t *testing.T) {
	xmlData := `<event><date>2025-01-01</date></event>`

	dateHook := func(s string) any {
		parsed, _ := time.Parse("2006-01-02", s)
		return parsed
	}

	m, _ := MapXML(strings.NewReader(xmlData), WithValueHook("date", dateHook))

	event := m["event"].(map[string]any)
	if _, ok := event["date"].(time.Time); !ok {
		t.Errorf("Hook falló: se esperaba time.Time")
	}
}

// ============================================================================
// 2. TEST DE ERRORES Y CASOS BORDE (UNHAPPY PATHS)
// ============================================================================

func TestMapXML_Errors(t *testing.T) {
	// XML Inválido (sin cerrar tag)
	xmlData := `<root><open>ups`

	_, err := MapXML(strings.NewReader(xmlData))
	if err == nil {
		t.Error("Se esperaba error con XML mal formado, pero no ocurrió")
	}
}

func TestQuery_Errors(t *testing.T) {
	xmlData := `<root><a>1</a></root>`
	m, _ := MapXML(strings.NewReader(xmlData))

	// Ruta inexistente
	_, err := Query(m, "root/b/c")
	if err == nil {
		t.Error("Query debió fallar con 'not found'")
	}

	// Filtro que no matchea
	_, err = Query(m, "root/a[val=999]")
	if err == nil {
		t.Error("Query con filtro inválido debió fallar")
	}
}

// ============================================================================
// 3. TEST DE QUERY (NAVEGACIÓN)
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
		t.Errorf("Query falló: %v, %v", err, res)
	}

	// QueryAll (Deep Search)
	results, err := QueryAll(m, "store/item/name")
	if len(results) != 2 {
		t.Errorf("QueryAll esperaba 2 items directos, obtuvo %d", len(results))
	}
}

// ============================================================================
// 4. TEST DE VALIDACIÓN
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
		t.Errorf("Validación esperaba 3 errores, obtuvo %d: %v", len(errs), errs)
	}
}

// ============================================================================
// 5. TEST DE STREAMING (DECODER & ENCODER)
// ============================================================================

func TestStreamingDecoder(t *testing.T) {
	xmlData := `
	<feed>
		<Entry><Id>1</Id></Entry>
		<Entry><Id>2</Id></Entry>
	</feed>`

	type Entry struct {
		Id int `xml:"Id"`
	}

	stream := NewStream[Entry](strings.NewReader(xmlData), "Entry")

	count := 0
	for item := range stream.Iter() {
		count++
		if item.Id != count {
			t.Errorf("Streaming orden incorrecto")
		}
	}
	if count != 2 {
		t.Errorf("Streaming esperaba 2 elementos")
	}
}

func TestEncoder_Features(t *testing.T) {
	data := map[string]any{
		"root": map[string]any{
			"-lang":     "es",
			"#comments": []string{" Test Comment "},
			"content": map[string]any{
				"#cdata": "<b>Bold</b>",
			},
		},
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf, WithPrettyPrint())
	if err := enc.Encode(data); err != nil {
		t.Fatalf("Encoder falló: %v", err)
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
			t.Errorf("Encoder output falta '%s'. Output:\n%s", check, output)
		}
	}
}

// ============================================================================
// 6. BENCHMARKS (PRUEBAS DE RENDIMIENTO)
// ============================================================================

// BenchmarkMapXML mide la velocidad de parsear a mapa en memoria
func BenchmarkMapXML(b *testing.B) {
	// Generamos un XML mediano
	xmlData := `<root>` + strings.Repeat(`<item>Data</item>`, 1000) + `</root>`
	r := strings.NewReader(xmlData)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Seek(0, 0) // Resetear reader
		MapXML(r)
	}
}

// BenchmarkStreaming mide la velocidad de leer elemento por elemento
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
