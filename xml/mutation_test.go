package xml

import (
	"testing"
)

func TestOrderedMap_Rename(t *testing.T) {
	m := NewMap()
	m.Put("oldKey", "value")

	err := m.Rename("oldKey", "newKey")
	if err != nil {
		t.Errorf("Rename failed: %v", err)
	}

	if m.Has("oldKey") {
		t.Error("oldKey should be gone")
	}
	if !m.Has("newKey") {
		t.Error("newKey should exist")
	}
	if m.Get("newKey") != "value" {
		t.Error("Value should be preserved")
	}

	// Test order preservation?
	// Implementation detail: Rename likely appends or modifies in place.
	// If the implementation of Rename preserves order, we can test it.
}

func TestOrderedMap_Move(t *testing.T) {
	m := NewMap()
	m.Set("a/b/c", "value")

	// Move a/b/c -> x/y/z
	err := m.Move("a/b/c", "x/y/z")
	if err != nil {
		t.Errorf("Move failed: %v", err)
	}
}
