package xml

import (
	"testing"
)

// ============================================================================
// FIXTURE DATA
// ============================================================================

func getHelperFixture() map[string]any {
	return map[string]any{
		"user": map[string]any{
			"name": "Alice",
			"role": "Admin",
			"meta": map[string]any{ // Nested map
				"login_count": 42,
			},
		},
		"tags": []any{"go", "xml", "parser"}, // Simple array
		"items": []any{
			map[string]any{"id": 1, "val": 100},
			map[string]any{"id": 2, "val": 200},
			map[string]any{"id": 3, "val": 300},
		},
	}
}

// ============================================================================
// 1. TEST SET
// ============================================================================

func TestSet(t *testing.T) {
	t.Run("Map Update", func(t *testing.T) {
		data := getHelperFixture()
		// Update existing
		if err := Set(data, "user/name", "Bob"); err != nil {
			t.Fatalf("Set failed: %v", err)
		}
		if got, _ := Query(data, "user/name"); got != "Bob" {
			t.Errorf("Set failed to update value. Got %v", got)
		}
	})

	t.Run("Map Create Intermediate", func(t *testing.T) {
		data := getHelperFixture()
		// Path "config/server/port" does not exist. Should create config -> server -> port
		if err := Set(data, "config/server/port", 8080); err != nil {
			t.Fatalf("Set create failed: %v", err)
		}
		val, _ := Query(data, "config/server/port")
		if val != 8080 {
			t.Errorf("Set failed to create deep path. Got %v", val)
		}
	})

	t.Run("Array Update", func(t *testing.T) {
		data := getHelperFixture()
		// Update tags[1] ("xml") -> "json"
		if err := Set(data, "tags[1]", "json"); err != nil {
			t.Fatalf("Set array failed: %v", err)
		}

		list := data["tags"].([]any)
		if list[1] != "json" {
			t.Errorf("Array update failed. Got %v", list[1])
		}
	})

	t.Run("Errors", func(t *testing.T) {
		data := getHelperFixture()

		// Error: Index out of bounds
		if err := Set(data, "tags[99]", "fail"); err == nil {
			t.Error("Expected error for out of bounds index, got nil")
		}

		// Error: Expecting array, found map
		if err := Set(data, "user[0]", "fail"); err == nil {
			t.Error("Expected error when treating map as array, got nil")
		}

		// Error: Expecting map, found array (cannot navigate through array without index)
		// "tags" is []any, we try to access "tags/something"
		if err := Set(data, "tags/something", "fail"); err == nil {
			t.Error("Expected error navigating through array as map, got nil")
		}
	})
}

// ============================================================================
// 2. TEST DELETE
// ============================================================================

func TestDelete(t *testing.T) {
	t.Run("Delete Map Key", func(t *testing.T) {
		data := getHelperFixture()

		if err := Delete(data, "user/role"); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		if _, err := Query(data, "user/role"); err == nil {
			t.Error("Key 'user/role' should have been deleted")
		}
	})

	t.Run("Delete Nested Map Key", func(t *testing.T) {
		data := getHelperFixture()
		if err := Delete(data, "user/meta/login_count"); err != nil {
			t.Fatalf("Delete nested failed: %v", err)
		}
		if _, err := Query(data, "user/meta/login_count"); err == nil {
			t.Error("Nested key should be gone")
		}
	})

	t.Run("Delete Array Index (Simple)", func(t *testing.T) {
		data := getHelperFixture()
		// tags: ["go", "xml", "parser"]
		// Delete index 1 ("xml")
		if err := Delete(data, "tags[1]"); err != nil {
			t.Fatalf("Delete array index failed: %v", err)
		}

		list := data["tags"].([]any)
		// Should now be length 2 and contain ["go", "parser"]
		if len(list) != 2 {
			t.Errorf("Array length mismatch. Want 2, got %d", len(list))
		}
		if list[1] != "parser" {
			t.Errorf("Array shift failed. Index 1 should be 'parser', got %v", list[1])
		}
	})

	t.Run("Delete Array Index (Complex)", func(t *testing.T) {
		data := getHelperFixture()
		// items has 3 objects. Delete items[0]
		if err := Delete(data, "items[0]"); err != nil {
			t.Fatalf("Delete complex array failed: %v", err)
		}

		list := data["items"].([]any)
		if len(list) != 2 {
			t.Errorf("Complex array length mismatch. Want 2, got %d", len(list))
		}
		// The previous item[1] (id:2) should now be item[0]
		first := list[0].(map[string]any)
		if first["id"] != 2 {
			t.Errorf("Complex array shift failed. New ID should be 2, got %v", first["id"])
		}
	})

	t.Run("Errors", func(t *testing.T) {
		data := getHelperFixture()

		// Delete non-existent (Idempotent - Should NOT error)
		if err := Delete(data, "user/ghost"); err != nil {
			t.Errorf("Delete non-existent key should be idempotent (no error), got: %v", err)
		}

		// Delete Out of bounds
		if err := Delete(data, "tags[99]"); err == nil {
			t.Error("Expected error for out of bounds delete")
		}
	})
}

// ============================================================================
// 3. TEST GET (Generics)
// ============================================================================

func TestGet(t *testing.T) {
	data := getHelperFixture()
	// Add an int that we will try to read as float
	data["price"] = 50

	t.Run("Get String", func(t *testing.T) {
		val, err := Get[string](data, "user/name")
		if err != nil {
			t.Fatalf("Get[string] error: %v", err)
		}
		if val != "Alice" {
			t.Errorf("Get value mismatch: %v", val)
		}
	})

	t.Run("Get Slice", func(t *testing.T) {
		val, err := Get[[]any](data, "tags")
		if err != nil {
			t.Fatalf("Get[[]any] error: %v", err)
		}
		if len(val) != 3 {
			t.Errorf("Get slice length mismatch: %d", len(val))
		}
	})

	t.Run("Get Float from Int (Auto-Conversion)", func(t *testing.T) {
		// "price" is int(50) in the map.
		// We request float64. The helper should convert it.
		val, err := Get[float64](data, "price")
		if err != nil {
			t.Fatalf("Get[float64] failed conversion: %v", err)
		}
		if val != 50.0 {
			t.Errorf("Get conversion mismatch: %v", val)
		}
	})

	t.Run("Type Mismatch Error", func(t *testing.T) {
		// Try to read a string ("Alice") as an int
		_, err := Get[int](data, "user/name")
		if err == nil {
			t.Error("Expected error for type mismatch (string -> int)")
		}
	})

	t.Run("Not Found Error", func(t *testing.T) {
		_, err := Get[string](data, "user/ghost")
		if err == nil {
			t.Error("Expected error for missing key")
		}
	})
}
