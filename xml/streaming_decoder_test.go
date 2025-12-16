package xml

import (
	"context"
	"strings"
	"testing"
)

func TestStreamingDecoder(t *testing.T) {
	// Caso Simple: Iter() normal
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
	for item := range stream.Iter() { // API Limpia
		count++
		if item.Id != count {
			t.Errorf("Streaming incorrect order")
		}
	}
	if count != 2 {
		t.Errorf("Streaming expected 2 elements")
	}
}

func TestStreamingDecoder_Context(t *testing.T) {
	// Caso Avanzado: Cancelación
	xmlData := `<feed>` + strings.Repeat(`<Entry><Id>1</Id></Entry>`, 1000) + `</feed>`

	type Entry struct {
		Id int `xml:"Id"`
	}

	stream := NewStream[Entry](strings.NewReader(xmlData), "Entry")

	// Cancelamos después de un tiempo muy corto
	ctx, cancel := context.WithCancel(context.Background())

	count := 0
	for range stream.IterWithContext(ctx) {
		count++
		if count == 10 {
			cancel() // ¡Cortamos el chorro!
		}
	}

	// Si leyera los 1000, fallaría. Debería leer ~10-11 (dependiendo de la carrera del select)
	if count > 20 {
		t.Errorf("Context cancellation failed. Read %d items, expected close to 10", count)
	}
}

func TestStreamingDecoder_ComplexAttributes(t *testing.T) {
	// Scenario: Parsing items with XML attributes and inner text (Mixed content)
	xmlData := `
    <catalog>
        <Book id="b1" lang="en">The Go Programming Language</Book>
        <Book id="b2" lang="es">El Quijote</Book>
    </catalog>`

	type Book struct {
		ID       string `xml:"id,attr"`
		Language string `xml:"lang,attr"`
		Title    string `xml:",chardata"`
	}

	stream := NewStream[Book](strings.NewReader(xmlData), "Book")

	var books []Book
	for b := range stream.Iter() {
		books = append(books, b)
	}

	// Assertions
	if len(books) != 2 {
		t.Fatalf("Expected 2 books, got %d", len(books))
	}
	if books[0].ID != "b1" || books[0].Title != "The Go Programming Language" {
		t.Errorf("First book mismatch: %+v", books[0])
	}
	if books[1].Language != "es" {
		t.Errorf("Second book attribute mismatch: %+v", books[1])
	}
}

func TestStreamingDecoder_NoMatches(t *testing.T) {
	// Scenario: The XML is valid, but the target tag does not exist.
	// The channel should close gracefully without yielding items.
	xmlData := `
    <root>
        <User>Alice</User>
        <User>Bob</User>
    </root>`

	// We are looking for "Product", but only "User" exists.
	stream := NewStream[string](strings.NewReader(xmlData), "Product")

	count := 0
	for range stream.Iter() {
		count++
	}

	if count != 0 {
		t.Errorf("Expected 0 matches, got %d", count)
	}
}

func TestStreamingDecoder_Malformed(t *testing.T) {
	// Scenario: The XML stream ends abruptly (network error, corrupted file).
	// The decoder should process valid items until the error and then stop gracefully.
	xmlData := `
    <feed>
        <Item>Value 1</Item>
        <Item>Value 2</Item>
        <Item>Val... [unexpected EOF]
    `

	type Item struct {
		Val string `xml:",chardata"`
	}

	stream := NewStream[Item](strings.NewReader(xmlData), "Item")

	count := 0
	for range stream.Iter() {
		count++
	}

	// It should recover the first 2 valid items before stopping.
	if count != 2 {
		t.Errorf("Expected 2 valid items before crash, got %d", count)
	}
}
