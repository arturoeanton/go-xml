package xml

import (
	"fmt"
	"strings"
)

// Set updates or inserts a value at a given path.
// It supports map keys ("user/name") and specific indices of existing arrays ("users/user[0]/name").
// Note: It creates intermediate map keys automatically, but it does NOT create arrays.
func Set(data any, path string, value any) error {
	parts := strings.Split(path, "/")
	var current any = data

	for i, part := range parts {
		isLast := i == len(parts)-1
		key, _, idx := parseSegment(part) // Reuse logic from main.go/query.go

		// 1. Current node must be a map or OrderedMap to lookup the 'key'
		var currentMap map[string]any
		var currentOM *OrderedMap
		var isOrdered bool

		if om, ok := current.(*OrderedMap); ok {
			currentOM = om
			isOrdered = true
		} else if m, ok := current.(map[string]any); ok {
			currentMap = m
		} else {
			return fmt.Errorf("cannot navigate '%s': current node is not a map, found %T", part, current)
		}

		// 2. Handle Array Access: "tags[1]"
		if idx >= 0 {
			var arrVal any
			var exists bool

			if isOrdered {
				arrVal = currentOM.Get(key)
				exists = arrVal != nil
			} else {
				arrVal, exists = currentMap[key]
			}

			if !exists {
				return fmt.Errorf("array '%s' not found", key)
			}
			list, ok := arrVal.([]any)
			if !ok {
				return fmt.Errorf("key '%s' exists but is not an array, found %T", key, arrVal)
			}
			if idx >= len(list) {
				return fmt.Errorf("index %d out of bounds for '%s'", idx, key)
			}

			if isLast {
				list[idx] = value
				return nil
			}

			// Navigate deeper into the array item
			current = list[idx]
			continue
		}

		// 3. Handle Standard Map Key: "user"
		if isLast {
			if isOrdered {
				currentOM.Put(key, value)
			} else {
				currentMap[key] = value
			}
			return nil
		}

		// If the next node does not exist, create it as a map (default to OrderedMap if parent is Ordered, else map)
		if isOrdered {
			if val := currentOM.Get(key); val == nil {
				newMap := NewMap()
				currentOM.Put(key, newMap)
				current = newMap
			} else {
				current = val
			}
		} else {
			if _, exists := currentMap[key]; !exists {
				currentMap[key] = make(map[string]any)
			}
			current = currentMap[key]
		}
	}
	return nil
}

// Delete removes a value or a complete node at the specified path.
// It supports deletion of map keys ("user/email") and array indices ("users/user[1]").
func Delete(data any, path string) error {
	parts := strings.Split(path, "/")

	// 1. Navigate to the PARENT of the element to delete.
	var current any = data
	lastIdx := len(parts) - 1

	for i := 0; i < lastIdx; i++ {
		part := parts[i]
		key, _, idx := parseSegment(part)

		// Current must be a map
		var val any
		var exists bool

		if om, ok := current.(*OrderedMap); ok {
			val = om.Get(key)
			exists = val != nil
		} else if m, ok := current.(map[string]any); ok {
			val, exists = m[key]
		} else {
			return fmt.Errorf("invalid path '%s': parent is not a map", part)
		}

		// If it's an array navigation ("tags[0]")
		if idx >= 0 {
			if !exists {
				return fmt.Errorf("path '%s' not found", part)
			}
			list, ok := val.([]any)
			if !ok {
				return fmt.Errorf("expected array at '%s', found %T", key, val)
			}
			if idx >= len(list) {
				return fmt.Errorf("index out of range at '%s': %d", part, idx)
			}
			current = list[idx] // Advance inside the array
			continue
		}

		// Standard map navigation
		if !exists {
			return fmt.Errorf("path '%s' not found", part)
		}
		current = val // Advance
	}

	// 2. Execute Deletion on the last segment
	targetPart := parts[lastIdx]
	key, _, idx := parseSegment(targetPart)

	// The 'current' is the container map
	if om, ok := current.(*OrderedMap); ok {
		// CASE A: Delete an element from an Array
		if idx >= 0 {
			val := om.Get(key)
			if val == nil {
				return nil // Idempotent
			}
			list, ok := val.([]any)
			if !ok {
				return fmt.Errorf("attempted to delete index from '%s' but it is not an array", key)
			}
			if idx >= len(list) {
				return fmt.Errorf("deletion index out of range: %d", idx)
			}
			// Delete from slice
			newList := append(list[:idx], list[idx+1:]...)
			om.Put(key, newList)
			return nil
		}
		// CASE B: Delete a Map key
		om.Remove(key)
		return nil

	} else if m, ok := current.(map[string]any); ok {
		// CASE A: Delete an element from an Array
		if idx >= 0 {
			val, exists := m[key]
			if !exists {
				return nil // Idempotent
			}
			list, ok := val.([]any)
			if !ok {
				return fmt.Errorf("attempted to delete index from '%s' but it is not an array", key)
			}
			if idx >= len(list) {
				return fmt.Errorf("deletion index out of range: %d", idx)
			}
			// Delete from slice
			newList := append(list[:idx], list[idx+1:]...)
			m[key] = newList
			return nil
		}
		// CASE B: Delete a Map key
		delete(m, key)
		return nil

	} else {
		return fmt.Errorf("cannot delete '%s', parent is not an accessible map", targetPart)
	}
}

// Get retrieves a value at the specified path and asserts it to type T.
// Usage: name, err := xml.Get[string](m, "user/name")
func Get[T any](data any, path string) (T, error) {
	var zero T

	// 1. Reuse the Query engine
	val, err := Query(data, path)
	if err != nil {
		return zero, err
	}

	// 2. Type Assertion
	typedVal, ok := val.(T)
	if !ok {
		// Special Case: int -> float64 conversion
		if vInt, isInt := val.(int); isInt {
			if vFloat, okFloat := any(float64(vInt)).(T); okFloat {
				return vFloat, nil
			}
		}
		return zero, fmt.Errorf("Get: value at '%s' is type %T, expected %T", path, val, zero)
	}

	return typedVal, nil
}
