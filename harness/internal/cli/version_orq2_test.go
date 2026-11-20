package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionOrq2(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	var out, errb bytes.Buffer
	if code := Run([]string{"version"}, nil, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if got := out.String(); got != "2.0.0-pilot revision=unknown modified=unknown\n" {
		t.Fatalf("stdout %q", got)
	}
	out.Reset()
	errb.Reset()
	Run(nil, nil, &out, &errb)
	if first, _, _ := strings.Cut(errb.String(), "\n"); first != "orq2 2.0.0-pilot · harness de orquestador-2" {
		t.Fatalf("usage header %q", first)
	}
}
