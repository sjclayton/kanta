package ui

import "testing"

func TestPlaceholderGrid_shapeAndFirstKey(t *testing.T) {
	g := (&App{}).PlaceholderGrid()
	if g.Width != 10 || g.Height == 0 {
		t.Fatalf("unexpected grid shape: %dx%d", g.Width, g.Height)
	}
	if len(g.Keys) == 0 {
		t.Fatalf("expected non-empty keys")
	}
	if g.Keys[0].Label != "1" || g.Keys[0].Row != 0 || g.Keys[0].Col != 0 {
		t.Fatalf("unexpected first key: %+v", g.Keys[0])
	}
}
