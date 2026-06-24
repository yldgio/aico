package container

import "testing"

func TestNameDeterministic(t *testing.T) {
	a := Name("pi", "/home/dev/project")
	b := Name("pi", "/home/dev/project")
	if a != b {
		t.Fatalf("name not deterministic: %q vs %q", a, b)
	}
}

func TestNameFormat(t *testing.T) {
	got := Name("pi", "/tmp/x")
	// aico-pi-<8 hex>
	if len(got) != len("aico-pi-")+8 {
		t.Fatalf("unexpected name length: %q", got)
	}
	if got[:8] != "aico-pi-" {
		t.Fatalf("unexpected prefix: %q", got)
	}
}

func TestNameVariesByPathAndAgent(t *testing.T) {
	if Name("pi", "/a") == Name("pi", "/b") {
		t.Fatal("different paths produced same name")
	}
	if Name("pi", "/a") == Name("claude", "/a") {
		t.Fatal("different agents produced same name")
	}
}
