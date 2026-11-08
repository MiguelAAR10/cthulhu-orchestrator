package board

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"orq/internal/plan"
	"orq/internal/slice"
	"orq/internal/stops"
	"orq/internal/workers"
)

var now = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

func openSlice(t *testing.T, orq, nn, title, fase string) {
	t.Helper()
	os.MkdirAll(filepath.Join(orq, "slices"), 0o755)
	raw, _ := json.Marshal(slice.Estado{NN: nn, Intento: 1, Fase: fase, Titulo: title, Test: "t", Abierto: now.Add(-time.Hour)})
	os.WriteFile(filepath.Join(orq, "slices", nn+".json"), raw, 0o644)
}

func savePlan(t *testing.T, orq, nn string, ts ...plan.Tarea) {
	t.Helper()
	if err := plan.Guardar(plan.Path(orq, nn), &plan.Plan{Slice: nn, Tareas: ts}); err != nil {
		t.Fatal(err)
	}
}

func find(b Board, id string) *Slice {
	for i := range b.Slices {
		if b.Slices[i].ID == id {
			return &b.Slices[i]
		}
	}
	return nil
}

// Acceptance 1.
func TestPctWeighted(t *testing.T) {
	orq := t.TempDir()
	openSlice(t, orq, "01", "Customers can pay", slice.Implementando)
	savePlan(t, orq, "01",
		plan.Tarea{ID: "T1", Dificultad: "alta", Estado: "hecha", Que: "checkout page"},
		plan.Tarea{ID: "T2", Dificultad: "baja", Estado: "pendiente", Que: "receipt email"},
		plan.Tarea{ID: "T3", Dificultad: "media", Estado: "pendiente"})
	b, err := Build(orq, now)
	if err != nil {
		t.Fatal(err)
	}
	s := find(b, "S-01")
	if s.Pct == nil || *s.Pct != 50 || strings.Join(s.Missing, ",") != "receipt email,T3" || s.Color != "blue" {
		t.Fatalf("%+v", s)
	}
	if b.Overall.Pct == nil || *b.Overall.Pct != 50 || b.Overall.Measured != 1 {
		t.Fatalf("%+v", b.Overall)
	}
}

// Acceptance 2.
func TestNoPlanIsUnknownGrey(t *testing.T) {
	orq := t.TempDir()
	openSlice(t, orq, "01", "a", slice.Implementando)
	openSlice(t, orq, "02", "b", slice.Implementando)
	savePlan(t, orq, "01", plan.Tarea{ID: "T1", Estado: "hecha"})
	b, _ := Build(orq, now)
	s := find(b, "S-02")
	if s.Pct != nil || s.Color != "grey" || b.Overall.Measured != 1 || b.Overall.Total != 2 {
		t.Fatalf("%+v %+v", s, b.Overall)
	}
	txt := Text(b, false)
	if !strings.Contains(txt, "unknown") || !strings.Contains(txt, "1 of 2 slices measured") {
		t.Fatal(txt)
	}
	raw, _ := JSON(b)
	if !strings.Contains(string(raw), `"pct": null`) {
		t.Fatal(string(raw))
	}
}

// Acceptance 3.
func TestBlockingStopperFirstAndRed(t *testing.T) {
	orq := t.TempDir()
	openSlice(t, orq, "01", "a", slice.Implementando)
	openSlice(t, orq, "02", "b", slice.Implementando)
	savePlan(t, orq, "01", plan.Tarea{ID: "T1"})
	savePlan(t, orq, "02", plan.Tarea{ID: "T1"})
	p := stops.Path(orq)
	stops.Add(p, stops.Raise{Type: "access", Severity: "fyi", Slice: "S-01", By: "w", Text: "fyi first raised"}, now.Add(-time.Hour))
	stops.Add(p, stops.Raise{Type: "api_key", Severity: "blocking", Slice: "S-02", Tasks: []string{"T1"}, By: "w", Text: "need key"}, now)
	b, _ := Build(orq, now)
	if b.Stoppers[0].Severity != "blocking" || find(b, "S-02").Color != "red" || b.Slices[0].ID != "S-02" {
		t.Fatalf("%+v", b)
	}
	txt := Text(b, false)
	if strings.Index(txt, "need key") > strings.Index(txt, "fyi first raised") || strings.Index(txt, "need key") > strings.Index(txt, "OVERALL") {
		t.Fatal(txt)
	}
	if !strings.Contains(txt, "BLOCKED") {
		t.Fatal("status word missing")
	}
}

// Acceptance 4.
func TestUnknownWorkerFieldsAreNull(t *testing.T) {
	orq := t.TempDir()
	workers.Save(orq, workers.State{ID: "w1", Slice: "S-01", Task: "T1", Phase: "running", Started: now.Add(-5 * time.Minute), Heartbeat: now})
	b, _ := Build(orq, now)
	raw, _ := JSON(b)
	var m struct {
		Workers []map[string]any `json:"workers"`
	}
	json.Unmarshal(raw, &m)
	w := m.Workers[0]
	if v, ok := w["context_pct"]; !ok || v != nil {
		t.Fatal(string(raw))
	}
	if v, ok := w["tests"]; !ok || v != nil {
		t.Fatal(string(raw))
	}
	txt := Text(b, false)
	if !strings.Contains(txt, "tests unknown") || !strings.Contains(txt, "context unknown") || !strings.Contains(txt, "budget: none") {
		t.Fatal(txt)
	}
}

// Acceptance 5 (render part; the file write is tested in cli).
func TestHTMLStatic(t *testing.T) {
	orq := t.TempDir()
	openSlice(t, orq, "01", "see http://example.com <b>", slice.Implementando)
	b, _ := Build(orq, now)
	h := HTML(b)
	if strings.Contains(h, "<script") || strings.Contains(h, "http") || !strings.Contains(h, "&lt;b&gt;") {
		t.Fatal(h)
	}
}

// Acceptance 6.
func TestVelocity(t *testing.T) {
	orq := t.TempDir()
	workers.AppendTaskEvent(orq, "S-01", "T1", "done", "w", now.Add(-2*time.Hour))
	b, _ := Build(orq, now)
	if b.Velocity.TasksPerHour != nil {
		t.Fatal("one event must be unknown")
	}
	workers.AppendTaskEvent(orq, "S-01", "T2", "done", "w", now.Add(-time.Hour))
	workers.AppendTaskEvent(orq, "S-01", "T2", "reopened", "w", now.Add(-time.Hour))
	b, _ = Build(orq, now)
	if b.Velocity.TasksPerHour == nil || *b.Velocity.TasksPerHour != 1 {
		t.Fatalf("%+v", b.Velocity)
	}
}

func TestClosedAcceptedIsGreen100AndRework(t *testing.T) {
	orq := t.TempDir()
	reg, _ := json.Marshal(slice.Registro{NN: "03", Intento: 2, Titulo: "done thing", Result: "aceptado"})
	os.WriteFile(filepath.Join(orq, "slices.jsonl"), append(reg, '\n'), 0o644)
	workers.AppendTaskEvent(orq, "S-03", "T1", "reopened", "w", now)
	b, _ := Build(orq, now)
	s := find(b, "S-03")
	if s == nil || s.Color != "green" || s.Pct == nil || *s.Pct != 100 || s.Rework != 2 {
		t.Fatalf("%+v", s)
	}
}

func TestAmberOnStuckWorker(t *testing.T) {
	orq := t.TempDir()
	openSlice(t, orq, "01", "a", slice.Implementando)
	savePlan(t, orq, "01", plan.Tarea{ID: "T1", Estado: "en_curso", Worker: "w1"})
	pid := 999999
	workers.Save(orq, workers.State{ID: "w1", Slice: "S-01", Task: "T1", Phase: "running", PID: &pid, Host: hostname(), Started: now, Heartbeat: now})
	b, _ := Build(orq, now)
	s := find(b, "S-01")
	if s.Color != "amber" || strings.Join(s.Who, ",") != "w1" {
		t.Fatalf("%+v", s)
	}
}

func TestFrontsGroupedInJSON(t *testing.T) {
	orq := t.TempDir()
	openSlice(t, orq, "01", "Customers can pay", slice.Implementando)
	p := &plan.Plan{Slice: "01",
		Fronts: []plan.Front{{ID: "F1", Goal: "pay", Paths: []string{"pay"}}, {ID: "F2", Goal: "see receipt", Paths: []string{"web"}}},
		Tareas: []plan.Tarea{
			{ID: "a", Front: "F1", Estado: "hecha", Dificultad: "media"},
			{ID: "b", Front: "F1", Estado: "pendiente", Dificultad: "media"},
			{ID: "c", Front: "F2", Estado: "en_curso", Worker: "w1", Dificultad: "media"},
		}}
	if err := plan.Guardar(plan.Path(orq, "01"), p); err != nil {
		t.Fatal(err)
	}
	b, _ := Build(orq, now)
	raw, _ := JSON(b)
	var got struct {
		Slices []struct {
			Fronts []struct {
				ID    string    `json:"id"`
				Goal  string    `json:"goal"`
				Tasks TaskCount `json:"tasks"`
				IDs   []string  `json:"task_ids"`
			} `json:"fronts"`
		} `json:"slices"`
	}
	json.Unmarshal(raw, &got)
	fs := got.Slices[0].Fronts
	if len(fs) != 2 || fs[0].ID != "F1" || fs[0].Tasks.Done != 1 || fs[0].Tasks.Total != 2 ||
		strings.Join(fs[0].IDs, ",") != "a,b" || fs[1].Goal != "see receipt" || fs[1].Tasks.Total != 1 {
		t.Fatalf("%s", raw)
	}
}
