package xml

import (
	"fmt"
	"strings"
)

// Set actualiza o inserta un valor en una ruta dada.
// Soporta mapas ("user/name") y arrays existentes ("users/user[0]/name").
// Nota: No crea arrays automáticamente, solo claves de mapa.
func Set(data map[string]any, path string, value any) error {
	parts := strings.Split(path, "/")
	var current any = data

	for i, part := range parts {
		isLast := i == len(parts)-1
		key, _, _, idx := parseSegment(part) // Reusamos la lógica de main.go

		// 1. Manejo de Arrays [n]
		if idx >= 0 {
			// El padre debe ser un array
			list, ok := current.([]any)
			if !ok {
				return fmt.Errorf("la ruta '%s' requiere un array, se encontró %T", part, current)
			}
			if idx >= len(list) {
				return fmt.Errorf("índice fuera de rango en '%s': %d", part, idx)
			}

			if isLast {
				list[idx] = value
				return nil
			}
			current = list[idx] // Avanzamos
			continue
		}

		// 2. Manejo de Mapas
		m, ok := current.(map[string]any)
		if !ok {
			return fmt.Errorf("no se puede navegar por '%s', el padre no es un mapa", part)
		}

		if isLast {
			m[key] = value // Asignamos el valor
			return nil
		}

		// Si no existe el siguiente nodo, lo creamos como mapa
		if _, exists := m[key]; !exists {
			m[key] = make(map[string]any)
		}
		current = m[key] // Avanzamos
	}
	return nil
}

// Delete elimina un valor o un nodo completo en la ruta especificada.
// Soporta eliminación de claves de mapa ("user/email") y de índices de array ("users/user[1]").
func Delete(data map[string]any, path string) error {
	parts := strings.Split(path, "/")

	// 1. Navegación hasta el PADRE del elemento a borrar
	// Necesitamos quedarnos un nivel antes para poder borrar la clave o el índice.
	var current any = data
	lastIdx := len(parts) - 1

	for i := 0; i < lastIdx; i++ {
		part := parts[i]
		key, _, _, idx := parseSegment(part) // Usamos el mismo helper que en Set/Query

		// A. Navegación por Array
		if idx >= 0 {
			// El padre debe ser un array para que nosotros seamos un elemento [n]
			list, ok := current.([]any)
			if !ok {
				return fmt.Errorf("ruta '%s' invalida: se esperaba array, se encontró %T", part, current)
			}
			if idx >= len(list) {
				return fmt.Errorf("índice fuera de rango en '%s': %d", part, idx)
			}
			current = list[idx] // Avanzamos al objeto dentro del array
			continue
		}

		// B. Navegación por Mapa
		m, ok := current.(map[string]any)
		if !ok {
			return fmt.Errorf("ruta '%s' invalida: el padre no es un mapa", part)
		}

		val, exists := m[key]
		if !exists {
			return fmt.Errorf("ruta '%s' no encontrada", part)
		}
		current = val // Avanzamos
	}

	// 2. Ejecutar el Borrado en el último segmento
	targetPart := parts[lastIdx]
	key, _, _, idx := parseSegment(targetPart)

	// El 'current' actual es el contenedor (mapa) que tiene el elemento a borrar
	parentMap, ok := current.(map[string]any)
	if !ok {
		// Caso borde: si el path era "items[0]/name", el loop nos dejó en el objeto items[0].
		// Pero si el path era "items[0]", el loop no corrió y 'current' es data.
		return fmt.Errorf("no se puede borrar '%s', el padre no es un mapa accesible", targetPart)
	}

	// CASO A: Borrar un elemento de un Array (ej: "telefonos[1]")
	if idx >= 0 {
		// Obtenemos el array del mapa padre
		val, exists := parentMap[key]
		if !exists {
			return nil // Ya no existe, "borrado exitoso" (idempotente)
		}
		list, ok := val.([]any)
		if !ok {
			return fmt.Errorf("se intentó borrar índice de '%s' pero no es un array", key)
		}
		if idx >= len(list) {
			return fmt.Errorf("índice de borrado fuera de rango: %d", idx)
		}

		// Truco de Go para borrar de un slice: append(lo_anterior, lo_siguiente...)
		// Esto es ineficiente en arrays gigantes (O(N)), pero seguro para XMLs normales.
		newList := append(list[:idx], list[idx+1:]...)

		// IMPORTANTE: Debemos reasignar el slice modificado al mapa padre
		parentMap[key] = newList
		return nil
	}

	// CASO B: Borrar una clave de Mapa (ej: "email")
	delete(parentMap, key)

	return nil
}

// Get recupera un valor en la ruta especificada y lo convierte al tipo T.
// Uso: nombre, err := xml.Get[string](mapa, "user/name")
func Get[T any](data map[string]any, path string) (T, error) {
	var zero T // Valor cero del tipo (ej: "" para string, 0 para int)

	// 1. Reusamos el motor de Query (que ya sabe navegar)
	val, err := Query(data, path)
	if err != nil {
		return zero, err
	}

	// 2. Verificación de Tipo (Type Assertion Segura)
	typedVal, ok := val.(T)
	if !ok {
		// Caso especial: Si piden float64 pero viene int (muy común en JSON/XML)
		// intentamos convertirlo amablemente.
		if vInt, isInt := val.(int); isInt {
			// Si T es float64, convertimos el int
			if vFloat, okFloat := any(float64(vInt)).(T); okFloat {
				return vFloat, nil
			}
		}

		return zero, fmt.Errorf("Get: el valor en '%s' es de tipo %T, se esperaba %T", path, val, zero)
	}

	return typedVal, nil
}
