package mysqltest

import (
	"strings"
	"testing"
)

func TestRandomIdentifierIsSafeAndUnique(t *testing.T) {
	first, err := randomIdentifier("pinocchio_test_")
	if err != nil {
		t.Fatal(err)
	}
	second, err := randomIdentifier("pinocchio_test_")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{first, second} {
		if !strings.HasPrefix(value, "pinocchio_test_") {
			t.Fatalf("identifier %q missing prefix", value)
		}
		for _, r := range value {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
				t.Fatalf("identifier %q contains unsafe rune %q", value, r)
			}
		}
	}
	if first == second {
		t.Fatalf("random identifiers collided: %q", first)
	}
}

func TestQuoteIdentifier(t *testing.T) {
	if got := quoteIdentifier("safe_name"); got != "`safe_name`" {
		t.Fatalf("quoteIdentifier = %q", got)
	}
	if got := quoteIdentifier("a`b"); got != "`a``b`" {
		t.Fatalf("quoteIdentifier escaped value = %q", got)
	}
}

func TestEscapeSQLString(t *testing.T) {
	if got := escapeSQLString("a\\b'c"); got != "a\\\\b\\'c" {
		t.Fatalf("escapeSQLString = %q", got)
	}
}
