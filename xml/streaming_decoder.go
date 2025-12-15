package xml

import (
	"encoding/xml"
	"fmt"
	"io"
)

// ============================================================================
// 3. STREAMING DECODER (Feature 1: Alto Impacto)
// ============================================================================

// Stream permite iterar archivos gigantes sin cargarlos en memoria.
// Usa Generics para devolver structs tipados.
type Stream[T any] struct {
	decoder *xml.Decoder
	tagName string
}

// NewStream inicializa el iterador.
func NewStream[T any](r io.Reader, tagName string) *Stream[T] {
	return &Stream[T]{
		decoder: xml.NewDecoder(r),
		tagName: tagName,
	}
}

// Iter devuelve un canal. Debe usarse en un loop: for item := range stream.Iter()
func (s *Stream[T]) Iter() <-chan T {
	ch := make(chan T)
	go func() {
		defer close(ch)
		for {
			t, err := s.decoder.Token()
			if err == io.EOF {
				return
			}
			if err != nil {
				fmt.Printf("Stream error: %v\n", err) // Simple log
				return
			}

			if se, ok := t.(xml.StartElement); ok && se.Name.Local == s.tagName {
				var item T
				// DecodeElement hace el trabajo pesado de mapear XML a Struct
				if err := s.decoder.DecodeElement(&item, &se); err == nil {
					ch <- item
				}
			}
		}
	}()
	return ch
}
