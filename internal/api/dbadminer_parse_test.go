package api

import "testing"

func TestParseAdminerPath(t *testing.T) {
	tests := []struct {
		in     string
		slug   string
		access string
		rest   string
		ok     bool
	}{
		{"", "", "", "", false},
		{"edit", "", "", "", false},
		{"edit/foo", "", "", "", false},
		{"my_db", "my_db", adminerAccessRO, "", true},
		{"my_db/", "my_db", adminerAccessRO, "", true},
		{"my_db/edit", "my_db", adminerAccessFull, "", true},
		{"my_db/edit/", "my_db", adminerAccessFull, "", true},
		{"my_db/edit/api/info", "my_db", adminerAccessFull, "api/info", true},
		{"my_db/assets/x.js", "my_db", adminerAccessRO, "assets/x.js", true},
		{"parent_slug_notes/edit/api/query", "parent_slug_notes", adminerAccessFull, "api/query", true},
	}
	for _, tt := range tests {
		slug, access, rest, ok := parseAdminerPath(tt.in)
		if ok != tt.ok || slug != tt.slug || access != tt.access || rest != tt.rest {
			t.Fatalf("%q: got slug=%q access=%q rest=%q ok=%v", tt.in, slug, access, rest, ok)
		}
	}
}
