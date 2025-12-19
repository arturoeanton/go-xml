package xml

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// OrderedMap es una estructura de datos híbrida que mantiene el orden de inserción
// y ofrece utilidades avanzadas para manipulación de datos jerárquicos.
type OrderedMap struct {
	keys   []string       // Mantiene el orden
	values map[string]any // Mantiene la velocidad O(1)
}

// NewMap crea una nueva instancia de OrderedMap.
func NewMap() *OrderedMap {
	return &OrderedMap{
		keys:   make([]string, 0),
		values: make(map[string]any),
	}
}

// ---------------------------------------------------------
// 1. Fluent Setters & Core CRUD
// ---------------------------------------------------------

// Set inserta un valor en una ruta específica (ej: "Body/Auth/Token").
// Crea automáticamente los mapas anidados si no existen.
// Retorna el mismo mapa para encadenamiento (Fluent API).
func (om *OrderedMap) Set(path string, value any) *OrderedMap {
	parts := strings.Split(path, "/")
	lastIdx := len(parts) - 1
	current := om

	for i := 0; i < lastIdx; i++ {
		key := parts[i]
		// Si no existe o no es un OrderedMap, creamos uno nuevo
		if !current.Has(key) {
			newMap := NewMap()
			current.Put(key, newMap)
			current = newMap
		} else {
			// Si existe, verificamos que sea un OrderedMap
			val := current.Get(key)
			if nextMap, ok := val.(*OrderedMap); ok {
				current = nextMap
			} else {
				// Conflicto: existe pero no es un mapa. Sobrescribimos.
				newMap := NewMap()
				current.Put(key, newMap)
				current = newMap
			}
		}
	}

	current.Put(parts[lastIdx], value)
	return om
}

// Put inserta un par clave-valor directamente en este nivel.
func (om *OrderedMap) Put(key string, value any) {
	if _, exists := om.values[key]; !exists {
		om.keys = append(om.keys, key)
	}
	om.values[key] = value
}

// Get recupera un valor directo de este nivel.
func (om *OrderedMap) Get(key string) any {
	return om.values[key]
}

// Has verifica si una clave existe en este nivel.
func (om *OrderedMap) Has(key string) bool {
	_, exists := om.values[key]
	return exists
}

// Remove elimina una clave y mantiene la consistencia del orden.
func (om *OrderedMap) Remove(key string) {
	if _, exists := om.values[key]; !exists {
		return
	}
	delete(om.values, key)
	for i, k := range om.keys {
		if k == key {
			om.keys = append(om.keys[:i], om.keys[i+1:]...)
			break
		}
	}
}

// Len retorna la cantidad de claves en este mapa.
func (om *OrderedMap) Len() int {
	return len(om.keys)
}

// ---------------------------------------------------------
// 2. Navegación y Lectura Segura (Zero-Panic)
// ---------------------------------------------------------

// GetPath navega por la estructura usando "Key/SubKey".
func (om *OrderedMap) GetPath(path string) any {
	parts := strings.Split(path, "/")
	var current any = om

	for _, key := range parts {
		if omNode, ok := current.(*OrderedMap); ok {
			if !omNode.Has(key) {
				return nil
			}
			current = omNode.Get(key)
		} else if mapNode, ok := current.(map[string]any); ok {
			val, ok := mapNode[key]
			if !ok {
				return nil
			}
			current = val
		} else {
			return nil
		}
	}
	return current
}

// GetNode retorna un *OrderedMap en la ruta especificada.
// Retorna nil si no existe o no es un OrderedMap.
func (om *OrderedMap) GetNode(path string) *OrderedMap {
	val := om.GetPath(path)
	if node, ok := val.(*OrderedMap); ok {
		return node
	}
	return nil // Retorna nil, no panic
}

// List retorna un slice de *OrderedMap.
// Es "smart": si el nodo es único, lo envuelve en un slice.
// Si es un slice de any, filtra los que sean OrderedMap.
func (om *OrderedMap) List(path string) []*OrderedMap {
	val := om.GetPath(path)
	if val == nil {
		return []*OrderedMap{}
	}

	result := make([]*OrderedMap, 0)

	switch v := val.(type) {
	case *OrderedMap:
		result = append(result, v)
	case []*OrderedMap:
		return v
	case []any:
		for _, item := range v {
			if node, ok := item.(*OrderedMap); ok {
				result = append(result, node)
			}
		}
	}
	return result
}

// String obtiene un string de la ruta, retornando "" si falla.
func (om *OrderedMap) String(path string) string {
	val := om.GetPath(path)
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// Int obtiene un int de la ruta, retornando 0 si falla.
func (om *OrderedMap) Int(path string) int {
	val := om.GetPath(path)
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case string:
		i, _ := strconv.Atoi(v)
		return i
	default:
		return 0
	}
}

// Float obtiene un float64 de la ruta, retornando 0.0 si falla.
func (om *OrderedMap) Float(path string) float64 {
	val := om.GetPath(path)
	if val == nil {
		return 0.0
	}
	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	default:
		return 0.0
	}
}

// Bool obtiene un bool de la ruta, retornando false si falla.
func (om *OrderedMap) Bool(path string) bool {
	val := om.GetPath(path)
	if val == nil {
		return false
	}
	switch v := val.(type) {
	case bool:
		return v
	case string:
		s := strings.ToLower(v)
		return s == "true" || s == "1" || s == "yes" || s == "on"
	case int:
		return v == 1
	default:
		return false
	}
}

// ---------------------------------------------------------
// 3. Utils & Iteration
// ---------------------------------------------------------

// Keys retorna las claves en el orden actual.
func (om *OrderedMap) Keys() []string {
	result := make([]string, len(om.keys))
	copy(result, om.keys)
	return result
}

// Sort ordena las claves alfabéticamente.
func (om *OrderedMap) Sort() {
	sort.Strings(om.keys)
}

// ForEach itera sobre el mapa en orden.
func (om *OrderedMap) ForEach(fn func(key string, value any) bool) {
	for _, k := range om.keys {
		if !fn(k, om.values[k]) {
			break
		}
	}
}

// ToMap convierte recursivamente a map[string]any (pierde orden).
func (om *OrderedMap) ToMap() map[string]any {
	result := make(map[string]any, len(om.keys))
	for _, k := range om.keys {
		val := om.values[k]
		result[k] = toNative(val)
	}
	return result
}

// Helper recursivo para ToMap
func toNative(val any) any {
	switch v := val.(type) {
	case *OrderedMap:
		return v.ToMap()
	case []*OrderedMap:
		list := make([]any, len(v))
		for i, item := range v {
			list[i] = item.ToMap()
		}
		return list
	case []any:
		list := make([]any, len(v))
		for i, item := range v {
			list[i] = toNative(item)
		}
		return list
	default:
		return v
	}
}

// ToJSON retorna la representación JSON del mapa preservando orden (indirectamente vía MarshalJSON).
func (om *OrderedMap) ToJSON() (string, error) {
	b, err := om.MarshalJSON()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (om *OrderedMap) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range om.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		keyBytes, _ := json.Marshal(k)
		buf.Write(keyBytes)
		buf.WriteByte(':')
		valBytes, err := json.Marshal(om.values[k])
		if err != nil {
			return nil, err
		}
		buf.Write(valBytes)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// ---------------------------------------------------------
// 4. Interoperabilidad XML (La pieza faltante)
// ---------------------------------------------------------

// MarshalXML implementa la interfaz xml.Marshaler.
// Esto permite que OrderedMap funcione nativamente con encoding/xml.
func (om *OrderedMap) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	var childrenKeys []string

	// 1. Separar Atributos (@) de Hijos
	// Es vital inyectar los atributos en el 'start' token ANTES de emitirlo.
	finalStart := start.Copy()

	for _, k := range om.keys {
		val := om.values[k]

		if strings.HasPrefix(k, "@") {
			// Es un atributo
			attrName := strings.TrimPrefix(k, "@")
			finalStart.Attr = append(finalStart.Attr, xml.Attr{
				Name:  xml.Name{Local: attrName},
				Value: fmt.Sprintf("%v", val),
			})
		} else {
			// Es contenido
			childrenKeys = append(childrenKeys, k)
		}
	}

	// 2. Emitir Start Element (con atributos)
	if err := e.EncodeToken(finalStart); err != nil {
		return err
	}

	// 3. Emitir Hijos (En Orden)
	for _, k := range childrenKeys {
		val := om.values[k]

		if k == "#text" {
			// Texto directo
			if err := e.EncodeToken(xml.CharData([]byte(fmt.Sprintf("%v", val)))); err != nil {
				return err
			}
			continue
		}

		// Hijo normal (Recursión automática manejada por Go)
		if err := e.EncodeElement(val, xml.StartElement{Name: xml.Name{Local: k}}); err != nil {
			return err
		}
	}

	// 4. Cerrar Elemento
	return e.EncodeToken(finalStart.End())
}

// ---------------------------------------------------------
// 5. Debug Helper
// ---------------------------------------------------------

// Dump retorna una representación string bonita (JSON Indented) de la estructura.
// Útil para logs: fmt.Println(resp.Dump())
func (om *OrderedMap) Dump() string {
	b, err := om.MarshalJSON()
	if err != nil {
		return fmt.Sprintf("<DumpError: %v>", err)
	}
	var out bytes.Buffer
	if err := json.Indent(&out, b, "", "  "); err != nil {
		return string(b) // Fallback a JSON minificado si falla la indentación
	}
	return out.String()
}
