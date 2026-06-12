package bug

import "testing"

func TestHasLabel(t *testing.T) {
	b := &Bug{Labels: []string{"customer-asked", "sev1", "mssql"}}

	cases := []struct {
		name   string
		labels []string
		want   bool
	}{
		{"single match", []string{"sev1"}, true},
		{"any-of match", []string{"sev2", "mssql"}, true},
		{"none match", []string{"sev2", "postgres"}, false},
		{"empty input", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := b.HasLabel(tc.labels...); got != tc.want {
				t.Fatalf("HasLabel(%v) = %v, want %v", tc.labels, got, tc.want)
			}
		})
	}
}
