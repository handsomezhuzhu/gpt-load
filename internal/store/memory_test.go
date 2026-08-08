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

func TestMemoryStoreLIndex(t *testing.T) {
	s := NewMemoryStore()

	key := "test:lindex"
	if err := s.LPush(key, "a", "b", "c"); err != nil {
		t.Fatalf("LPush failed: %v", err)
	}

	for index, want := range []string{"a", "b", "c"} {
		got, err := s.LIndex(key, int64(index))
		if err != nil {
			t.Fatalf("LIndex(%d) failed: %v", index, err)
		}
		if got != want {
			t.Fatalf("LIndex(%d) = %q, want %q", index, got, want)
		}
	}

	// Negative index counts from the tail
	if got, err := s.LIndex(key, -1); err != nil || got != "c" {
		t.Fatalf("LIndex(-1) = %q, %v; want %q", got, err, "c")
	}

	// Out-of-range indexes return ErrNotFound
	if _, err := s.LIndex(key, 3); !errors.Is(err, ErrNotFound) {
		t.Fatalf("LIndex(3) = %v, want ErrNotFound", err)
	}
	if _, err := s.LIndex(key, -4); !errors.Is(err, ErrNotFound) {
		t.Fatalf("LIndex(-4) = %v, want ErrNotFound", err)
	}

	// Missing/empty lists return ErrNotFound
	if _, err := s.LIndex("missing:list", 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("LIndex on missing key = %v, want ErrNotFound", err)
	}
	if err := s.LPush("empty:list"); err != nil {
		t.Fatalf("LPush failed: %v", err)
	}
	if _, err := s.LIndex("empty:list", 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("LIndex on empty list = %v, want ErrNotFound", err)
	}
}
