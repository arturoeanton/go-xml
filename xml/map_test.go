package xml

import (
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"
)

func TestOrderedMap_BasicOperations(t *testing.T) {
	m := NewMap()

	// Test Put & Order
	m.Put("Z", 1)
	m.Put("A", 2)
	m.Put("M", 3)

	if m.Len() != 3 {
		t.Errorf("Expected len 3, got %d", m.Len())
	}

	// Verificar orden de keys
	keys := m.Keys()
	if keys[0] != "Z" || keys[1] != "A" || keys[2] != "M" {
		t.Errorf("Order check failed. Got %v", keys)
	}

	// Test Get
	if val := m.Get("A"); val != 2 {
		t.Errorf("Get failed, expected 2, got %v", val)
	}

	// Test Update (Order shouldn't change)
	m.Put("Z", 99)
	keysAfterUpdate := m.Keys()
	if keysAfterUpdate[0] != "Z" {
		t.Error("Update changed key order!")
	}
	if m.Get("Z") != 99 {
		t.Error("Update failed value")
	}

	// Test Remove
	m.Remove("A")
	if m.Len() != 2 {
		t.Error("Remove failed len")
	}
	if m.Has("A") {
		t.Error("Remove failed, key still exists")
	}
	keysAfterRemove := m.Keys()
	if keysAfterRemove[0] != "Z" || keysAfterRemove[1] != "M" {
		t.Error("Remove broke order")
	}
}

func TestOrderedMap_JSON(t *testing.T) {
	m := NewMap()
	m.Put("name", "Arturo")
	m.Put("age", 40)
	m.Put("city", "BUE")

	// JSON estándar en Go ordena alfabéticamente las keys de structs/maps.
	// Nuestro OrderedMap DEBE respetar el orden de inserción.
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	expected := `{"name":"Arturo","age":40,"city":"BUE"}`
	if string(b) != expected {
		t.Errorf("JSON Order failed.\nExpected: %s\nGot:      %s", expected, string(b))
	}
}

func TestOrderedMap_XML(t *testing.T) {
	m := NewMap()
	// Simulando estructura ARCA: Auth debe tener Token antes que Sign
	m.Put("Token", "ABC")
	m.Put("Sign", "XYZ")
	m.Put("@xmlns", "http://afip.gov.ar") // Atributo

	type Envelope struct {
		Body *OrderedMap `xml:"Body"`
	}
	env := Envelope{Body: m}

	b, err := xml.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	xmlStr := string(b)

	// Validar que el atributo se inyectó
	if !strings.Contains(xmlStr, `Body xmlns="http://afip.gov.ar"`) {
		t.Error("XML Attribute injection failed")
	}

	// Validar Orden estricto: Token antes de Sign
	tokenIdx := strings.Index(xmlStr, "<Token>ABC</Token>")
	signIdx := strings.Index(xmlStr, "<Sign>XYZ</Sign>")

	if tokenIdx == -1 || signIdx == -1 {
		t.Fatal("XML elements missing")
	}
	if tokenIdx > signIdx {
		t.Error("XML Order failed! Sign appeared before Token")
	}
}
func TestOrderedMap_AdvancedFeatures(t *testing.T) {
	// 1. Test Deep Access (GetPath)
	root := NewMap()
	auth := NewMap()
	auth.Put("Token", "12345")
	root.Put("Header", auth)

	val := root.GetPath("Header/Token")
	if val != "12345" {
		t.Errorf("GetPath failed. Expected '12345', got %v", val)
	}

	if root.GetPath("Header/NonExistent") != nil {
		t.Error("GetPath should return nil for missing keys")
	}

	// 2. Test Merge
	defaults := NewMap()
	defaults.Put("Timeout", 30)
	defaults.Put("Retries", 3)

	config := NewMap()
	config.Put("Timeout", 60)   // Override
	config.Put("User", "Admin") // New

	defaults.Merge(config)

	if defaults.Get("Timeout") != 60 {
		t.Error("Merge did not override existing key")
	}
	if defaults.Get("User") != "Admin" {
		t.Error("Merge did not add new key")
	}
	if defaults.Len() != 3 {
		t.Error("Merge len incorrect")
	}

	// 3. Test Sort
	unsorted := NewMap()
	unsorted.Put("Z", 1)
	unsorted.Put("A", 2)
	unsorted.Put("M", 3)

	unsorted.Sort()
	keys := unsorted.Keys()
	if keys[0] != "A" || keys[1] != "M" || keys[2] != "Z" {
		t.Errorf("Sort failed. Got %v", keys)
	}

	// 4. Test Clone
	original := NewMap()
	original.Put("A", 1)
	copyMap := original.Clone()

	copyMap.Put("A", 999) // Mutate copy

	if original.Get("A") == 999 {
		t.Error("Clone is linked to original! Mutation leaked.")
	}
}

func TestOrderedMap_ToMap(t *testing.T) {
	// Estructura Compleja: Root -> Child -> GrandChild
	grandChild := NewMap()
	grandChild.Put("Secret", "Code")

	child := NewMap()
	child.Put("Name", "Hijo")
	child.Put("Details", grandChild) // Anidado

	root := NewMap()
	root.Put("Type", "Padre")
	root.Put("Info", child) // Anidado

	// También probamos un slice mixto
	list := []any{NewMap(), "texto"}
	root.Put("List", list)

	// CONVERSIÓN
	native := root.ToMap()

	// Validaciones
	// 1. Root debe ser map[string]any
	if _, ok := native["Type"]; !ok {
		t.Error("Root key missing")
	}

	// 2. Child debe ser map[string]any (NO *OrderedMap)
	infoVal := native["Info"]
	nativeChild, ok := infoVal.(map[string]any)
	if !ok {
		t.Errorf("Nested child is not map[string]any, got %T", infoVal)
	}

	// 3. GrandChild debe ser map[string]any
	detailsVal := nativeChild["Details"]
	nativeGrandChild, ok := detailsVal.(map[string]any)
	if !ok {
		t.Errorf("Deep nested child is not map[string]any, got %T", detailsVal)
	}

	if nativeGrandChild["Secret"] != "Code" {
		t.Error("Deep value lost")
	}

	// 4. Slice debe estar limpio
	listVal := native["List"].([]any)
	if _, ok := listVal[0].(map[string]any); !ok {
		t.Error("Map inside slice was not converted")
	}
}
