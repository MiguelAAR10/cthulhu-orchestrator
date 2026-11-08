package cli

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"orq/internal/state"
	"orq/internal/workers"
)

func TestEnglishSliceLaunchWorksLikeLanzar(t *testing.T) {
	root, _ := sliceConFake(t, false)
	if _, errs, code := run(t, "slice", "launch", "--agent", "codex", "--difficulty", "low"); code != 0 {
		t.Fatalf("launch: %s", errs)
	}
	st, err := workers.Load(filepath.Join(root, ".orq"), "worker-codex-s01")
	if err != nil || st.Agent != "codex" || st.Phase != "running" {
		t.Fatalf("%+v %v", st, err)
	}
	if out, _, code := run(t, "slice", "status"); code != 0 || !strings.Contains(out, "S-01") {
		t.Fatalf("status: %d %s", code, out)
	}
}

func TestEnglishSliceLaunchGoesThroughAdmission(t *testing.T) {
	sliceConFake(t, false)
	state.WriteHalt(state.Dir(), "disk", time.Now())
	t.Setenv("ORQ_LANG", "")
	_, errs, code := run(t, "slice", "launch", "--agent", "codex", "--difficulty", "low")
	if code == 0 || !strings.Contains(errs, "HALT") {
		t.Fatalf("%d %s", code, errs)
	}
}

func TestEnglishFlagsAndSubcommands(t *testing.T) {
	proyectoVis(t)
	t.Setenv("ORQ_ROL", "")
	t.Setenv("ORQ_WORKER_ID", "w1")
	_, errs, code := run(t, "slice", "exam", "tests/x.sh", "--approved", "m1")
	if code == 0 || !strings.Contains(errs, "H-01") {
		t.Fatalf("exam: %d %s", code, errs)
	}
	t.Setenv("ORQ_WORKER_ID", "")
	if out, errs, code := run(t, "plan", "--slice", "04", "ready"); code != 0 || !strings.Contains(out, "ready: T1, T2") {
		t.Fatalf("plan ready: %d %s %s", code, out, errs)
	}
	if _, errs, code := run(t, "plan", "--slice", "04", "show"); code != 0 {
		t.Fatalf("plan show: %s", errs)
	}
}

func TestEnglishUsageByDefault(t *testing.T) {
	t.Setenv("ORQ_LANG", "")
	_, errs, _ := run(t)
	if !strings.Contains(errs, "orq slice open") || !strings.Contains(errs, "orq slice launch") {
		t.Fatal(errs)
	}
	t.Setenv("ORQ_LANG", "es")
	_, errs, _ = run(t)
	if !strings.Contains(errs, "orq slice abrir") {
		t.Fatal(errs)
	}
}

func TestNormalizeArgs(t *testing.T) {
	cases := [][2]string{
		{"slice launch --agent a --difficulty low --task T1 --zone front --paths a,b --scope x", "slice lanzar --agente a --dificultad baja --tarea T1 --zona front --rutas a,b --alcance x"},
		{"slice --slice 03 close --accepted", "slice --slice 03 cerrar --aceptado"},
		{"slice open 01 --title=x --no-scouts r --strict", "slice abrir 01 --titulo=x --sin-scouts r --estricto"},
		{"autonomy free --scope src/**", "autonomy free --scope src/**"},
		{"session open planner", "sesion abrir planificador"},
		{"msg send --to atena --type closing hi", "msg send --para atena --tipo cierre hi"},
		{"msg read", "msg leer"},
		{"learn promote L1", "aprender promover L1"},
		{"metrics --since 2026-01-01", "metricas --desde 2026-01-01"},
		{"handoff-note orchestrator", "relevo orquestador"},
		{"slice abrir 01 --titulo x", "slice abrir 01 --titulo x"},
	}
	for _, c := range cases {
		if got := strings.Join(normalizeArgs(strings.Fields(c[0])), " "); got != c[1] {
			t.Errorf("%q → %q, want %q", c[0], got, c[1])
		}
	}
}
