package captureattach

import "testing"

func TestOutboundURLPath(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"http://localhost:4000/api/users", "/api/users"},
		{"https://localhost:4000/api/users", "/api/users"},
		{"/api/users", "/api/users"},
		{"http://localhost:4000", "/"},
	}
	for _, tc := range tests {
		if got := outboundURLPath(tc.url); got != tc.want {
			t.Fatalf("outboundURLPath(%q) = %q want %q", tc.url, got, tc.want)
		}
	}
}
