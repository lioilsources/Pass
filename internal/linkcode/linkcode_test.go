package linkcode

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		c, err := Generate()
		if err != nil {
			t.Fatal(err)
		}
		if len(c) != Length {
			t.Fatalf("len %d", len(c))
		}
		for _, r := range c {
			if !strings.ContainsRune(Alphabet, r) {
				t.Fatalf("bad rune %q in %q", r, c)
			}
		}
		if seen[c] {
			t.Fatalf("duplicate %q", c)
		}
		seen[c] = true
	}
}

func TestNormalizeDisplay(t *testing.T) {
	for _, in := range []string{"abcd-efgh", "ABCD EFGH", " abcdefgh\n", "abcd_efgh"} {
		got, err := Normalize(in)
		if err != nil || got != "ABCDEFGH" {
			t.Fatalf("Normalize(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "ABC", "ABCD-EFG0", "ABCDEFGHJ", "abcd-efg1"} {
		if _, err := Normalize(in); err == nil {
			t.Fatalf("Normalize(%q) accepted", in)
		}
	}
	if Display("ABCDEFGH") != "ABCD-EFGH" {
		t.Fatal("display")
	}
	if Hash("ABCDEFGH") == Hash("ABCDEFGJ") || len(Hash("x")) != 64 {
		t.Fatal("hash")
	}
}
