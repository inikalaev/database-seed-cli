package cli

import (
	"testing"

	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

func TestParseListInputString(t *testing.T) {
	got, err := parseListInput("pending, active, cancelled", seedapi.SetupString)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].(string) != "pending" || got[2].(string) != "cancelled" {
		t.Fatalf("got %v", got)
	}
}

func TestParseListInputInt(t *testing.T) {
	got, err := parseListInput("1, 2, 3", seedapi.SetupInt)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].(int) != 1 || got[2].(int) != 3 {
		t.Fatalf("got %v", got)
	}
}

func TestParseListInputFloat(t *testing.T) {
	got, err := parseListInput("0.5, 1.25", seedapi.SetupFloat)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].(float64) != 1.25 {
		t.Fatalf("got %v", got)
	}
}

func TestParseListInputBool(t *testing.T) {
	got, err := parseListInput("true, false, true", seedapi.SetupBool)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].(bool) != true || got[1].(bool) != false {
		t.Fatalf("got %v", got)
	}
}

func TestParseListInputEmpty(t *testing.T) {
	if _, err := parseListInput(" , , ", seedapi.SetupString); err == nil {
		t.Fatal("empty CSV must error")
	}
}

func TestParseListInputBadElement(t *testing.T) {
	if _, err := parseListInput("1,notanumber", seedapi.SetupInt); err == nil {
		t.Fatal("bad int element must error")
	}
}

func TestFloatValidator(t *testing.T) {
	if err := floatValidator("3.14"); err != nil {
		t.Fatalf("valid float: %v", err)
	}
	if err := floatValidator("nan-not-here"); err == nil {
		t.Fatal("invalid float must error")
	}
	if err := floatValidator(3.14); err == nil {
		t.Fatal("non-string input must error")
	}
}
