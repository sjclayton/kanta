package parser

import "testing"

func TestParse_nilInput_returnsEmptyDocNoError(t *testing.T) {
	doc, err := Parse(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Raw) != 0 {
		t.Fatalf("expected empty doc, got %d bytes", len(doc.Raw))
	}
}
