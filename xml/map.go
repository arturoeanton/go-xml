package xml

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"sort"
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
// 1. CRUD Básico (El 20% original)
// ---------------------------------------------------------

func (om *OrderedMap) Put(key string, value any) {
	if _, exists := om.values[key]; !exists {
		om.keys = append(om.keys, key)
	}
	om.values[key] = value
}

func (om *OrderedMap) Get(key string) any {
	return om.values[key]
}

func (om *OrderedMap) Has(key string) bool {
	_, exists := om.values[key]
	return exists
}

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

func (om *OrderedMap) Len() int {
	return len(om.keys)
}

func (om *OrderedMap) Keys() []string {
	result := make([]string, len(om.keys))
	copy(result, om.keys)
	return result
}

// ---------------------------------------------------------
// 2. Funcionalidades Avanzadas (El +20% Nuevo)
// ---------------------------------------------------------

// GetPath permite recuperar un valor anidado usando sintaxis de path ("Auth/Token").
// Soporta tanto OrderedMap como map[string]any anidados.
func (om *OrderedMap) GetPath(path string) any {
	parts := strings.Split(path, "/")
	var current any = om

	for _, key := range parts {
		switch t := current.(type) {
		case *OrderedMap:
			if !t.Has(key) {
				return nil
			}
			current = t.Get(key)
		case map[string]any:
			val, ok := t[key]
			if !ok {
				return nil
			}
			current = val
		default:
			return nil // No se puede navegar más profundo
		}
	}
	return current
}

// ForEach itera sobre el mapa en el orden garantizado ejecutando la función fn.
// Retorna false si se quiere detener la iteración prematuramente.
func (om *OrderedMap) ForEach(fn func(key string, value any) bool) {
	for _, k := range om.keys {
		if !fn(k, om.values[k]) {
			break
		}
	}
}

// Sort ordena las claves alfabéticamente (A-Z).
// Útil para Canonicalización XML (firmas digitales) o visualización consistente.
func (om *OrderedMap) Sort() {
	sort.Strings(om.keys)
}

// Merge copia todos los elementos de 'other' dentro de este mapa.
// Si la clave ya existe, se sobrescribe el valor y se mantiene la posición original.
// Si es nueva, se agrega al final.
func (om *OrderedMap) Merge(other *OrderedMap) {
	if other == nil {
		return
	}
	other.ForEach(func(k string, v any) bool {
		om.Put(k, v)
		return true
	})
}

// Clone crea una copia superficial (shallow copy) del mapa.
// Las claves y la estructura se duplican, pero los valores punteros se comparten.
func (om *OrderedMap) Clone() *OrderedMap {
	newMap := NewMap()
	newMap.keys = make([]string, len(om.keys))
	copy(newMap.keys, om.keys)

	for k, v := range om.values {
		newMap.values[k] = v
	}
	return newMap
}

// ---------------------------------------------------------
// 3. Serialización (JSON/XML)
// ---------------------------------------------------------

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

func (om *OrderedMap) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	// Estrategia: Detectar atributos (@) y pasarlos al start element
	var childrenKeys []string

	// Pre-scan para separar atributos de hijos
	for _, k := range om.keys {
		if strings.HasPrefix(k, "@") {
			attrName := strings.TrimPrefix(k, "@")
			start.Attr = append(start.Attr, xml.Attr{
				Name:  xml.Name{Local: attrName},
				Value: fmt.Sprintf("%v", om.values[k]),
			})
		} else {
			childrenKeys = append(childrenKeys, k)
		}
	}

	if err := e.EncodeToken(start); err != nil {
		return err
	}

	for _, k := range childrenKeys {
		val := om.values[k]
		if k == "#text" {
			if err := e.EncodeToken(xml.CharData([]byte(fmt.Sprintf("%v", val)))); err != nil {
				return err
			}
			continue
		}
		if err := e.EncodeElement(val, xml.StartElement{Name: xml.Name{Local: k}}); err != nil {
			return err
		}
	}
	return e.EncodeToken(start.End())
}

// ---------------------------------------------------------
// 4. Interoperabilidad (Bridge to Native)
// ---------------------------------------------------------

// ToMap convierte el OrderedMap (y todos sus hijos anidados)
// a un map[string]any nativo de Go.
// ADVERTENCIA: Al hacer esto, se pierde la garantía de orden en el resultado.
func (om *OrderedMap) ToMap() map[string]any {
	result := make(map[string]any, len(om.keys))

	// Iteramos usando keys para mantener un orden lógico de procesamiento,
	// aunque el mapa de destino sea desordenado.
	for _, k := range om.keys {
		val := om.values[k]
		result[k] = toNative(val)
	}
	return result
}

// toNative es un helper recursivo para limpiar estructuras anidadas
func toNative(val any) any {
	switch v := val.(type) {
	case *OrderedMap:
		return v.ToMap() // Recursión
	case []any:
		// Si es un slice, hay que revisar cada elemento
		newList := make([]any, len(v))
		for i, item := range v {
			newList[i] = toNative(item)
		}
		return newList
	case []*OrderedMap:
		// Caso especial: Slice tipado de OrderedMaps
		newList := make([]any, len(v))
		for i, item := range v {
			newList[i] = item.ToMap()
		}
		return newList
	default:
		return v // Tipos primitivos (int, string, bool) pasan igual
	}
}
