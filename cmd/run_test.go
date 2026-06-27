package cmd

import "testing"

func TestIsAffirmative(t *testing.T) {
	yes := []string{"y", "Y", "yes", "YES", " yes \n", "y\r\n"}
	no := []string{"", "n", "no", "nope", "x", " \n", "yeah"}
	for _, s := range yes {
		if !isAffirmative(s) {
			t.Errorf("isAffirmative(%q) = false, want true", s)
		}
	}
	for _, s := range no {
		if isAffirmative(s) {
			t.Errorf("isAffirmative(%q) = true, want false", s)
		}
	}
}
