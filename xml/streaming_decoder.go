package xml

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
)

// ============================================================================
// 3. STREAMING DECODER (Feature: High Performance / Large Files)
// ============================================================================

// Stream allows iterating over huge XML files efficiently without loading
// the entire content into memory.
// It leverages Go Generics to yield typed structs directly.
type Stream[T any] struct {
	decoder *xml.Decoder
	tagName string
}

// NewStream initializes a new streaming iterator for a specific XML tag.
// r: The input reader (file, http body, etc).
// tagName: The local name of the XML element to iterate over (e.g., "Item", "Entry").
// opts: Variadic options (e.g., EnableLegacyCharsets)
func NewStream[T any](r io.Reader, tagName string, opts ...Option) *Stream[T] {
	// 1. Procesar configuración
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	decoder := xml.NewDecoder(r)

	// 2. Inyectar el CharsetReader si la opción está activa
	if cfg.useCharsetReader {
		// Aquí usamos la función charsetReader definida en charset.go
		decoder.CharsetReader = charsetReader
	}

	return &Stream[T]{
		decoder: decoder,
		tagName: tagName,
	}
}

// Iter returns a read-only channel of items of type T.
// It is a convenience wrapper around IterWithContext using context.Background().
//
// Usage:
//
//	stream := xml.NewStream[MyStruct](reader, "MyTag")
//	for item := range stream.Iter() {
//	    // process item
//	}
func (s *Stream[T]) Iter() <-chan T {
	return s.IterWithContext(context.Background())
}

// IterWithContext returns a channel of items, respecting the provided Context.
// Use this method if you need to cancel the streaming process early or handle timeouts.
//
// Usage:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
//	defer cancel()
//	for item := range stream.IterWithContext(ctx) { ... }
func (s *Stream[T]) IterWithContext(ctx context.Context) <-chan T {
	ch := make(chan T)
	go func() {
		defer close(ch)
		for {
			// 1. Check cancellation before work
			select {
			case <-ctx.Done():
				return
			default:
			}

			t, err := s.decoder.Token()
			if err == io.EOF {
				return
			}
			if err != nil {
				// In a production environment, consider an error channel.
				fmt.Printf("Stream error: %v\n", err)
				return
			}

			if se, ok := t.(xml.StartElement); ok && se.Name.Local == s.tagName {
				var item T
				if err := s.decoder.DecodeElement(&item, &se); err == nil {
					// 2. Blocking Send with Context Awareness
					// Prevents goroutine leak if the receiver stops reading.
					select {
					case ch <- item:
						// OK
					case <-ctx.Done():
						return // Abort
					}
				}
			}
		}
	}()
	return ch
}
