package store

import (
	"errors"
	"testing"
)

func TestMemoryStoreLFirst(t *testing.T) {
	s := NewMemoryStore()

	key := "test:list"
	if err := s.LPush(key, "a", "b", "c"); err != nil {
		t.Fatalf("LPush failed: %v", err)
	}

	// MemoryStore prepends values in argument order, so the head is "a"
	first, err := s.LFirst(key)
	if err != nil {
		t.Fatalf("LFirst failed: %v", err)
	}
	if first != "a" {
		t.Fatalf("LFirst = %q, want %q", first, "a")
	}

	// LFirst must not rotate the list
	firstAgain, err := s.LFirst(key)
	if err != nil {
		t.Fatalf("LFirst failed: %v", err)
	}
	if firstAgain != "a" {
		t.Fatalf("LFirst after peek = %q, want still %q", firstAgain, "a")
	}
}

func TestMemoryStoreLFirstEmpty(t *testing.T) {
	s := NewMemoryStore()

	if _, err := s.LFirst("missing:list"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("LFirst on missing key = %v, want ErrNotFound", err)
	}

	if err := s.LPush("empty:list"); err != nil {
		t.Fatalf("LPush failed: %v", err)
	}
	if _, err := s.LFirst("empty:list"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("LFirst on empty list = %v, want ErrNotFound", err)
	}
}

func TestMemoryStoreRotateAfterLFirst(t *testing.T) {
	s := NewMemoryStore()

	key := "test:rotate"
	if err := s.LPush(key, "a", "b", "c"); err != nil {
		t.Fatalf("LPush failed: %v", err)
	}

	// Rotate pops the tail and pushes it to the head
	// list: [a, b, c] -> pop "c" -> [c, a, b]
	rotated, err := s.Rotate(key)
	if err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}
	if rotated != "c" {
		t.Fatalf("Rotate = %q, want %q", rotated, "c")
	}

	// After rotation the new head is the previously rotated key
	first, err := s.LFirst(key)
	if err != nil {
		t.Fatalf("LFirst failed: %v", err)
	}
	if first != "c" {
		t.Fatalf("LFirst after rotate = %q, want %q", first, "c")
	}
}
