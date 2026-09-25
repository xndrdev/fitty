package httpapi

import "testing"

func TestProgressPhotoFiltersAndCursor(t *testing.T) {
	for _, view := range []string{"front", "side", "back", "other"} {
		if !validProgressView(view) {
			t.Errorf("rejected %s", view)
		}
	}
	for _, view := range []string{"", "all", "Front", "left"} {
		if validProgressView(view) {
			t.Errorf("accepted %s", view)
		}
	}
	id := "FFAEF089-2073-4782-8E46-8456BDAED193"
	date, parsed, valid := progressCursor("2024-02-29:" + id)
	if !valid || date != "2024-02-29" || parsed != "ffaef089-2073-4782-8e46-8456bdaed193" {
		t.Fatal("valid cursor changed")
	}
	for _, cursor := range []string{"", "2024-02-29", "2023-02-29:" + id, "1899-12-31:" + id, "2024-02-29:invalid", "2024-02-29:" + id + ":extra"} {
		if _, _, valid := progressCursor(cursor); valid {
			t.Errorf("accepted %s", cursor)
		}
	}
}
