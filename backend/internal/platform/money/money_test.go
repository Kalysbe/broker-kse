package money

import "testing"

func TestParseRejectsFloatishNaN(t *testing.T) {
	if _, err := Parse("not-a-number"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := Parse("-1"); err == nil {
		t.Fatal("negative rejected")
	}
	a, err := Parse("1000.25")
	if err != nil || !a.Equal(Must("1000.25")) {
		t.Fatal(a, err)
	}
}
