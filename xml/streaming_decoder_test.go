package xml

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

type streamItem struct {
	ID   int    `xml:"id,attr"`
	Name string `xml:"name"`
}

func TestStream_Iter_Basic(t *testing.T) {
	data := `<root>
		<Item id="1"><name>Alice</name></Item>
		<Other>skip me</Other>
		<Item id="2"><name>Bob</name></Item>
	</root>`

	stream := NewStream[streamItem](strings.NewReader(data), "Item")

	var got []streamItem
	for item := range stream.Iter() {
		got = append(got, item)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 items, got %d: %+v", len(got), got)
	}
	if got[0].ID != 1 || got[0].Name != "Alice" {
		t.Errorf("item 0 = %+v, want {ID:1 Name:Alice}", got[0])
	}
	if got[1].ID != 2 || got[1].Name != "Bob" {
		t.Errorf("item 1 = %+v, want {ID:2 Name:Bob}", got[1])
	}
}

func TestStream_IterWithContext_AlreadyCancelled(t *testing.T) {
	data := `<root><Item id="1"><name>A</name></Item></root>`
	stream := NewStream[streamItem](strings.NewReader(data), "Item")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	count := 0
	for range stream.IterWithContext(ctx) {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 items with a pre-cancelled context, got %d", count)
	}
}

func TestStream_IterWithContext_CancelStopsIterationEarly(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("<root>")
	const total = 1000
	for i := 0; i < total; i++ {
		fmt.Fprintf(&sb, `<Item id="%d"><name>n%d</name></Item>`, i, i)
	}
	sb.WriteString("</root>")

	stream := NewStream[streamItem](strings.NewReader(sb.String()), "Item")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	count := 0
	for range stream.IterWithContext(ctx) {
		count++
		if count == 1 {
			cancel()
		}
	}
	if count == 0 {
		t.Fatal("expected at least 1 item before cancellation, got 0")
	}
	if count >= total {
		t.Errorf("expected cancellation to stop iteration well before the end, got all %d items", count)
	}
}

func TestStream_LegacyCharset(t *testing.T) {
	// ISO-8859-1 declared document with a raw Latin-1 byte (0xE9 = 'é').
	data := []byte("<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><root><Item id=\"1\"><name>Jos\xE9</name></Item></root>")

	stream := NewStream[streamItem](bytes.NewReader(data), "Item", EnableLegacyCharsets())

	var got []streamItem
	for item := range stream.Iter() {
		got = append(got, item)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 item, got %d", len(got))
	}
	if got[0].Name != "José" {
		t.Errorf("Name = %q, want José", got[0].Name)
	}
}

func TestStream_DecodeErrorClosesChannel(t *testing.T) {
	// Well-formed first Item, then an unclosed tag.
	data := `<root><Item id="1"><name>A</name></Item><Item id="2">unclosed`
	stream := NewStream[streamItem](strings.NewReader(data), "Item")

	var got []streamItem
	done := make(chan struct{})
	go func() {
		for item := range stream.Iter() {
			got = append(got, item)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("channel did not close after a decode error")
	}

	if len(got) != 1 {
		t.Errorf("expected exactly 1 valid item before the error, got %d: %+v", len(got), got)
	}
}
