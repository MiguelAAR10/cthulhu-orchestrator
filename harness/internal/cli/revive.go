package cli

// orq revive [ROL]: paquete de contexto compacto para retomar un rol desde el estado de .orq.
// Determinista y sin LLM: mismo estado, misma salida.

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"orq/internal/canal"
	"orq/internal/memoria"
	"orq/internal/plan"
	"orq/internal/slice"
	"orq/internal/stops"
)

const reviveMsgs = 10

func runRevive(args []string, stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	rol := ""
	if len(args) > 0 {
		rol = args[0]
	} else {
		rol = def(p.rolActual(), "owner")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# orq revive · role: %s\n", rol)

	ab, _ := p.slices().Abiertos()
	slices.SortFunc(ab, func(x, y *slice.Estado) int { return strings.Compare(x.NN, y.NN) })
	b.WriteString("\n## Active slices\n")
	if len(ab) == 0 {
		b.WriteString("(none)\n")
	}
	var last *slice.Recibo
	activo := ""
	for _, e := range ab {
		if activo == "" {
			activo = e.ID()
		}
		rev := "no plan"
		if pl, err := plan.Leer(plan.Path(p.dir, e.NN)); err == nil {
			ls, _ := plan.Listas(pl)
			rev = fmt.Sprintf("plan rev %s (agreed: %v) · ready: %s", corto(pl.Revision()), pl.Debatido, ids(ls))
		}
		fmt.Fprintf(&b, "- %s · %s (%s) · %s\n", e.ID(), e.Titulo, e.Fase, rev)
		for _, r := range []*slice.Recibo{e.Recibo, e.ReciboInt} {
			if r != nil && (last == nil || r.TS.After(last.TS)) {
				last = r
			}
		}
	}
	if last == nil {
		b.WriteString("last receipt: none\n")
	} else {
		fmt.Fprintf(&b, "last receipt: %s %s at %s · head %s · %s\n", last.Attempt, last.Resultado, last.TS.UTC().Format(time.RFC3339), corto(last.Head), def(last.Tipo, "worker"))
	}

	all, _ := stops.Load(stops.Path(p.dir))
	open := stops.Open(all)
	b.WriteString("\n## Pending stoppers\n")
	if len(open) == 0 {
		b.WriteString("(none)\n")
	}
	for _, s := range open {
		fmt.Fprintf(&b, "- %s %s/%s %s: %s\n", s.ID, s.Type, s.Severity, s.Slice, s.Text)
	}

	ms, _ := canal.Load(p.canalPath())
	var mios []canal.Msg
	for _, m := range canal.Vigentes(ms) {
		if m.Para == rol || m.De == rol {
			mios = append(mios, m)
		}
	}
	if len(mios) > reviveMsgs {
		mios = mios[len(mios)-reviveMsgs:]
	}
	fmt.Fprintf(&b, "\n## Channel (last %d unarchived for %s)\n", reviveMsgs, rol)
	if len(mios) == 0 {
		b.WriteString("(none)\n")
	}
	for _, m := range mios {
		fmt.Fprintf(&b, "- %s %s→%s %s [%s]: %s\n", m.ID, m.De, m.Para, m.Tipo, def(m.Estado, "pendiente"), m.Texto)
	}

	b.WriteString("\n## Memory\n")
	sel, fuera := memoria.Presupuesto(p.memVigente(), activo, presupuestoMem)
	if len(sel) == 0 {
		b.WriteString("(none)\n")
	} else {
		b.WriteString(strings.TrimSpace(memoria.Render(sel, "")) + "\n")
		if fuera > 0 {
			fmt.Fprintf(&b, "… %d more: orq mem ver\n", fuera)
		}
	}
	fmt.Fprintf(&b, "\nNext: `orq skill %s` for the guide, `orq board` for where things stand.\n", rol)
	fmt.Fprint(stdout, strings.ReplaceAll(b.String(), p.root, "."))
	return 0
}

func corto(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
