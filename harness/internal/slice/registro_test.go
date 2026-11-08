package slice

import (
	"testing"

	"orq/internal/worktrees"
)

func TestWorktreesDeSliceQuedanRegistrados(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	es, _ := worktrees.Leer(r.OrqDir)
	if len(es) != 1 || es[0].Path != e.Worktree || es[0].Kind != worktrees.KindSlice || es[0].Slice != "S-01" {
		t.Fatalf("%+v", es)
	}
	commitEn(t, e.Worktree, "total.txt", "170\n")
	r.Verificar("", t0)
	if _, _, _, err := r.Cerrar("", true, "", t0); err != nil {
		t.Fatal(err)
	}
	es, _ = worktrees.Leer(r.OrqDir)
	kinds := map[string]bool{}
	for _, x := range es {
		kinds[x.Kind] = true
	}
	if !kinds[worktrees.KindInt] {
		t.Fatalf("el worktree de integración no se registró: %+v", es)
	}
}
