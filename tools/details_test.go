package tools

import (
	"testing"
)

func TestDeduplicateStrings(t *testing.T) {
	got := deduplicateStrings([]string{"a", "b", "a", "c", "b"})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("deduplicateStrings: got %v, want %v", got, want)
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("deduplicateStrings[%d]: got %q, want %q", i, got[i], v)
		}
	}
}

func TestDeduplicateStrings_Empty(t *testing.T) {
	got := deduplicateStrings(nil)
	if len(got) != 0 {
		t.Errorf("deduplicateStrings(nil): got %v, want empty", got)
	}
}

func TestToStringSliceAny_Nil(t *testing.T) {
	got := toStringSliceAny(nil)
	if got != nil {
		t.Errorf("toStringSliceAny(nil): got %v, want nil", got)
	}
}

func TestToStringSliceAny_StringSlice(t *testing.T) {
	in := []string{"x", "y"}
	got := toStringSliceAny(in)
	if len(got) != 2 || got[0] != "x" || got[1] != "y" {
		t.Errorf("toStringSliceAny: got %v, want %v", got, in)
	}
}

func TestToStringSliceAny_WrongType(t *testing.T) {
	got := toStringSliceAny(42)
	if got != nil {
		t.Errorf("toStringSliceAny(int): got %v, want nil", got)
	}
}

func TestNewPubmedFetchDetailsTool_Construction(t *testing.T) {
	_, err := NewPubmedFetchDetailsTool(nil)
	if err != nil {
		t.Errorf("NewPubmedFetchDetailsTool: unexpected error: %v", err)
	}
}
