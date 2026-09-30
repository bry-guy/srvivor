package httpapi

import "testing"

func TestLoginReturnTo(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"/castawordle", "/castawordle"},
		{"/castawordle/123e4567-e89b-12d3-a456-426614174000", "/castawordle/123e4567-e89b-12d3-a456-426614174000"},
		{"https://evil.example", "/"},
		{"//evil.example", "/"},
		{"/castawordle/../../evil", "/"},
		{"/castawordle/bad-id", "/"},
		{"/castawordle?next=https://evil.example", "/"},
	} {
		if got := loginReturnTo(tc.input); got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.input, got, tc.want)
		}
	}
}
