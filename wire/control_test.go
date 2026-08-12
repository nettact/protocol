package wire

import "testing"

func TestHasCapability(t *testing.T) {
	cases := []struct {
		name string
		caps []string
		want bool
	}{
		{"declared", []string{"other", CapSequenceFloorV1, "another"}, true},
		{"declared with duplicates", []string{"other", CapSequenceFloorV1, CapSequenceFloorV1}, true},
		{"undeclared", []string{"other", "another"}, false},
		{"empty", nil, false},
	}
	for _, tc := range cases {
		if got := HasCapability(tc.caps, CapSequenceFloorV1); got != tc.want {
			t.Errorf("%s: HasCapability(%v) = %v, want %v", tc.name, tc.caps, got, tc.want)
		}
	}
}
