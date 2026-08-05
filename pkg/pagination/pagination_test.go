package pagination

import "testing"

func TestNormalizeBoundsPageAndSize(t *testing.T) {
	page := Normalize(0, 500)

	if page.Page != 1 {
		t.Fatalf("Page = %d, want 1", page.Page)
	}
	if page.PageSize != 100 {
		t.Fatalf("PageSize = %d, want 100", page.PageSize)
	}
	if page.Offset != 0 {
		t.Fatalf("Offset = %d, want 0", page.Offset)
	}
}
