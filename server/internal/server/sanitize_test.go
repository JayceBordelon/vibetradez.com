package server

import "testing"

func TestSanitizeForLog(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"clean", "Alice", "Alice"},
		{"outer whitespace", "  Alice  ", "Alice"},
		{"newline injection", "Foo\n[ERROR] fake", "Foo[ERROR] fake"},
		{"carriage return", "Foo\rBar", "FooBar"},
		{"CRLF pair", "Foo\r\nBar", "FooBar"},
		{"tab", "A\tB", "AB"},
		{"NUL byte", "A\x00B", "AB"},
		{"DEL byte", "A\x7fB", "AB"},
		{"mixed", "  Real\nName\t  ", "RealName"},
		{"unicode preserved", "Élise ñoño", "Élise ñoño"},
		{"empty", "", ""},
		{"only control", "\n\r\t", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeForLog(tc.in)
			if got != tc.want {
				t.Errorf("sanitizeForLog(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
