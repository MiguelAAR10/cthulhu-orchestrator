package qa

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Aislado: dónde corre QA y qué vigila.
//   - WorkerDir: el worktree del slice (o la raíz): su HEAD es lo que se prueba.
//   - MainRoot: el checkout principal del proyecto, vigilado entero (tracked y untracked) salvo .orq/.
//   - OrqDir: la carpeta .orq; de ella se vigila config.json por hash.
type Aislado struct {
	WorkerDir, MainRoot, OrqDir string
}

// Vigilancia es lo que el recibo guarda de la ejecución aislada.
type Vigilancia struct {
	Copy         string    `json:"copy"`                    // worktree desechable donde corrieron las capas (ya borrado)
	Main         TreeCheck `json:"main"`                    // checkout principal antes/después
	Worker       TreeCheck `json:"worker"`                  // worktree del worker antes/después
	ConfigBefore string    `json:"config_before,omitempty"` // sha256 de .orq/config.json
	ConfigAfter  string    `json:"config_after,omitempty"`
}

// RunAislado corre la pirámide en un `git worktree add --detach` desechable en el HEAD del worker,
// fuera del checkout. Fotografía el checkout principal, el worktree del worker y .orq/config.json
// antes y después; también la copia. Cualquier cambio (fuera de los informes permitidos, y de .orq/
// en el checkout principal, que orq mismo escribe) deja el recibo invalid. La copia se borra después.
// Si WorkerDir no es un repo git, cae a RunVigilado en el sitio.
func RunAislado(a Aislado, nn string, layers []Layer, crit string, run Runner, now time.Time, permitidos []string) Receipt {
	head := FotoArbol(a.WorkerDir, permitidos).Head
	if head == "" {
		return RunVigilado(a.WorkerDir, nn, layers, crit, run, now, permitidos)
	}
	cfgAntes := hashArchivo(filepath.Join(a.OrqDir, "config.json"))
	mainAntes := fotoMain(a.MainRoot, permitidos)
	workerAntes := FotoArbol(a.WorkerDir, permitidos)

	base, err := os.MkdirTemp("", "orq-qa-")
	if err != nil {
		return invalido(nn, head, crit, now, "cannot create the QA copy: "+err.Error())
	}
	copia := filepath.Join(base, "wt")
	defer quitarCopia(a.WorkerDir, base, copia)
	if out, err := exec.Command("git", "-C", a.WorkerDir, "worktree", "add", "--detach", "-q", copia, head).CombinedOutput(); err != nil {
		return invalido(nn, head, crit, now, fmt.Sprintf("git worktree add: %v %s", err, strings.TrimSpace(string(out))))
	}
	rec := RunVigilado(copia, nn, layers, crit, run, now, permitidos)
	rec.Head = head
	var cambios []string
	if rec.Tree != nil {
		for _, c := range rec.Tree.Changed {
			cambios = append(cambios, "copy: "+c)
		}
	}
	v := &Vigilancia{Copy: copia, ConfigBefore: cfgAntes, ConfigAfter: hashArchivo(filepath.Join(a.OrqDir, "config.json"))}
	v.Main = TreeCheck{Before: mainAntes, After: fotoMain(a.MainRoot, permitidos)}
	v.Main.Changed = Diferencias(v.Main.Before, v.Main.After)
	v.Worker = TreeCheck{Before: workerAntes, After: FotoArbol(a.WorkerDir, permitidos)}
	v.Worker.Changed = Diferencias(v.Worker.Before, v.Worker.After)
	for _, c := range v.Main.Changed {
		cambios = append(cambios, "main checkout: "+c)
	}
	if a.WorkerDir != a.MainRoot {
		for _, c := range v.Worker.Changed {
			cambios = append(cambios, "worker worktree: "+c)
		}
	}
	if v.ConfigBefore != v.ConfigAfter {
		cambios = append(cambios, ".orq/config.json changed")
	}
	rec.Isolation = v
	if rec.Tree == nil {
		rec.Tree = &TreeCheck{}
	}
	rec.Tree.Changed = cambios
	if len(cambios) > 0 {
		rec.Result = "invalid"
	}
	return rec
}

// fotoMain: el checkout principal sin .orq/ (orq escribe ahí latidos, canal y recibos).
func fotoMain(root string, permitidos []string) Arbol {
	a := FotoArbol(root, permitidos)
	var st []string
	for _, s := range a.Status {
		if len(s) > 3 && (strings.HasPrefix(s[3:], ".orq/") || s[3:] == ".orq") {
			continue
		}
		st = append(st, s)
	}
	a.Status = st
	return a
}

func hashArchivo(p string) string {
	raw, err := os.ReadFile(p)
	if err != nil {
		return "absent"
	}
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

// quitarCopia borra solo la copia que creó RunAislado: comprueba que vive bajo base (su carpeta
// temporal) antes de `git worktree remove --force`, y después poda el registro de worktrees.
func quitarCopia(repo, base, copia string) {
	if rel, err := filepath.Rel(base, copia); err != nil || strings.HasPrefix(rel, "..") || !strings.HasPrefix(filepath.Base(base), "orq-qa-") {
		return
	}
	exec.Command("git", "-C", repo, "worktree", "remove", "--force", copia).Run()
	os.RemoveAll(base)
	exec.Command("git", "-C", repo, "worktree", "prune").Run()
}

func invalido(nn, head, crit string, now time.Time, motivo string) Receipt {
	return Receipt{Slice: id(nn), Head: head, Criticality: normalizar(crit), Result: "invalid", TS: now.UTC().Truncate(time.Second),
		Tree: &TreeCheck{Changed: []string{motivo}}}
}
