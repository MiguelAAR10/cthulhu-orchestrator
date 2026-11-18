package slice

// Un worker trabaja en un clon independiente: su propio .git, sin objetos compartidos ni enlaces
// duros con el repo principal. Su trabajo vuelve como un bundle que orq verifica y trae a
// refs/orq/sNN/. orq no corre git con su directorio en un clon: lo que lee del clon se lee dentro
// de la caja, y solo sale un archivo.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"orq/internal/limits"
	"orq/internal/orqdir"
	"orq/internal/sandbox"
)

// MarcaClon es el archivo que orq deja en el .git de cada clon de worker (el nombre vive en orqdir).
const MarcaClon = orqdir.MarcaClon

// Marca es lo que el gate necesita saber dentro de un clon, sin ver el .orq del repo principal.
type Marca struct {
	Root       string   `json:"root"`
	Slice      string   `json:"slice"`
	Rama       string   `json:"rama"`
	Bundle     []string `json:"bundle"`
	Protegidos []string `json:"protegidos,omitempty"`
}

// LeerMarca busca la marca subiendo desde dir, también a través de repos anidados dentro del clon.
// ok=false si dir no está en un clon de worker. Un dir vacío o relativo no se puede ubicar: cuenta
// como worker (ok=true, marca vacía).
func LeerMarca(dir string) (Marca, bool) {
	if !filepath.IsAbs(dir) {
		return Marca{}, true
	}
	for d := filepath.Clean(dir); ; d = filepath.Dir(d) {
		if raw, err := os.ReadFile(filepath.Join(d, ".git", MarcaClon)); err == nil {
			var m Marca
			return m, json.Unmarshal(raw, &m) == nil
		}
		if filepath.Dir(d) == d {
			return Marca{}, false
		}
	}
}

// ControlPorDefecto son rutas que activan código o cambian reglas en las sesiones de confianza.
// Un worker no las cambia salvo que el alcance del slice las nombre (aprobado por Atena u Owner).
var ControlPorDefecto = []string{".claude/", ".codex/", ".envrc", ".githooks/", ".gitattributes", ".orq/"}

// protegidos son los patrones que un worker no cambia: los de limits.json más los de control.
func (r Repo) protegidos() []string {
	p := slices.Clone(ControlPorDefecto)
	if l, err := limits.Load(limits.DefaultPath()); err == nil {
		p = append(p, l.Protected...)
	} else {
		p = append(p, "tests/aceptacion/")
	}
	return p
}

// coincide dice si la ruta f cae en el patrón: un directorio («tests/aceptacion/», en cualquier nivel)
// o un archivo exacto.
func coincide(f, patron string) bool {
	f = filepath.ToSlash(filepath.Clean(f))
	p := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(patron)), "./")
	return f == p || strings.HasPrefix(f, p+"/") || strings.Contains("/"+f+"/", "/"+p+"/")
}

// crearClon deja en dir un clon independiente con la rama dada (que ya existe en el repo principal).
func (r Repo) crearClon(e *Estado, dir, rama string) error {
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	// --no-hardlinks: los objetos se copian; el worker no comparte ni un inodo con el repo principal.
	if _, err := r.git("clone", "-q", "--no-hardlinks", "--single-branch", "--no-tags", "--branch", rama, r.Root, dir); err != nil {
		return err
	}
	cfg := filepath.Join(dir, ".git", "config")
	// La configuración del clon se escribe como archivo, sin entrar en él. Sin remoto: su trabajo sale por bundle.
	for _, kv := range [][2]string{{"user.name", r.identidad("user.name", "orq worker")}, {"user.email", r.identidad("user.email", "worker@orq.local")}} {
		if _, err := r.git("config", "-f", cfg, kv[0], kv[1]); err != nil {
			return err
		}
	}
	for _, sec := range []string{"remote.origin", "branch." + rama} {
		if _, code, err := r.gitCodigo("config", "-f", cfg, "--remove-section", sec); err != nil || code > 1 && code != 128 {
			return errors.Join(err, fmt.Errorf("no se pudo quitar %s de la configuración del clon", sec))
		}
	}
	return r.marcar(e, dir, rama)
}

// marcar escribe (o actualiza) la marca del clon con el paquete del examen vigente.
func (r Repo) marcar(e *Estado, dir, rama string) error {
	raw, err := json.Marshal(Marca{Root: r.Root, Slice: e.NN, Rama: rama, Bundle: e.Bundle, Protegidos: r.protegidos()})
	if err != nil {
		return err
	}
	return orqdir.WriteAtomic(filepath.Join(dir, ".git", MarcaClon), raw)
}

func (r Repo) identidad(clave, defecto string) string {
	if v, err := r.git("config", "--get", clave); err == nil && v != "" {
		return v
	}
	return defecto
}

// esClon dice si dir es un clon de worker hecho por orq.
func esClon(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git", MarcaClon))
	return err == nil
}

// Ingesta es lo que salió de un clon.
type Ingesta struct {
	Punta string // commit de la rama del worker, ya en el repo principal
	Sucio string // archivos sin commit que el clon dice tener (aviso: no decide nada)
	Nuevo bool   // trajo commits que el repo principal no tenía en esa referencia
}

// LimiteIngesta acota el git que lee un clon.
const LimiteIngesta = 5 * time.Minute

// MaxBundle acota el bundle que orq acepta de un clon.
const MaxBundle = 2 << 30

// ingerir trae al repo principal los commits de la rama de un clon, en refs/orq/sNN/<nombre>.
//
//	en la caja (clon de solo lectura, sin red):  git bundle create → archivo en una carpeta de orq
//	en el repo principal:                        git bundle verify, git fetch con fsck
//
// La punta tiene que descender del rojo del slice. El «sucio» es lo que el git del clon contesta
// dentro de la caja: sirve para avisar al worker, no para aceptar nada.
func (r Repo) ingerir(e *Estado, clon, rama, nombre string) (Ingesta, error) {
	var in Ingesta
	ref := refWorker(e.NN, nombre)
	if !esClon(clon) {
		return in, fmt.Errorf("%s no es un clon de worker hecho por orq", clon)
	}
	fuera, err := os.MkdirTemp("", "orq-ingesta-")
	if err != nil {
		return in, err
	}
	defer os.RemoveAll(fuera)
	const guion = `git bundle create "$1/w.bundle" "refs/heads/$2" "^$3" >"$1/bundle.log" 2>&1; echo $? >"$1/rc"
git status --porcelain >"$1/sucio" 2>/dev/null; exit 0`
	if _, _, _, err := r.enCaja(sandbox.Spec{RO: []string{clon}, RW: []string{fuera}, Cwd: clon, Env: entornoExamen()},
		LimiteIngesta, nil, "sh", "-c", guion, "sh", fuera, rama, e.SHARojo); err != nil {
		return in, err
	}
	// Ya no corre nada del clon: lo que quedó en la carpeta no cambia mientras se lee.
	if raw, err := leerSeguro(fuera, "sucio", MaxReporte); err == nil {
		in.Sucio = strings.TrimSpace(string(raw))
	}
	rc, err := leerSeguro(fuera, "rc", 16)
	if err != nil {
		return in, fmt.Errorf("el git del clon %s no dejó resultado (¿se cortó?): %w", clon, err)
	}
	antes, _, err := r.gitCodigo("rev-parse", "-q", "--verify", ref)
	if err != nil {
		return in, err
	}
	if strings.TrimSpace(string(rc)) != "0" {
		log, _ := leerSeguro(fuera, "bundle.log", MaxReporte)
		if strings.Contains(string(log), "empty bundle") { // nada commiteado después del rojo
			if antes == "" {
				antes = e.SHARojo
			}
			in.Punta = antes
			return in, nil
		}
		return in, fmt.Errorf("no se pudo empaquetar la rama %s del clon %s: %s", rama, clon, cola(string(log), 5))
	}
	bundle := filepath.Join(fuera, "w.bundle")
	fi, err := os.Lstat(bundle)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > MaxBundle {
		return in, fmt.Errorf("el bundle del clon %s no es un archivo regular de menos de %d bytes", clon, int64(MaxBundle))
	}
	if _, err := r.git("bundle", "verify", "-q", bundle); err != nil {
		return in, fmt.Errorf("el bundle del clon %s no verifica: %w", clon, err)
	}
	if _, err := r.git("-c", "transfer.fsckObjects=true", "fetch", "-q", "--no-tags", bundle, "+refs/heads/"+rama+":"+ref); err != nil {
		return in, fmt.Errorf("no se pudo traer el bundle del clon %s: %w", clon, err)
	}
	punta, err := r.git("rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return in, err
	}
	if ok, err := r.esAncestro(e.SHARojo, punta); err != nil {
		return in, err
	} else if !ok {
		return in, errors.Join(fmt.Errorf("la rama %s del clon %s no desciende del rojo %s: el worker reescribió la historia. No se usa", rama, clon, Corto(e.SHARojo)), r.borrarRef(ref))
	}
	in.Punta, in.Nuevo = punta, punta != antes
	return in, nil
}

// puntaDelSlice es el commit que se verifica y se integra: la rama slice/NN del repo principal.
// Sin equipo, antes trae lo commiteado en el clon del slice y avanza esa rama (solo hacia adelante).
func (r Repo) puntaDelSlice(e *Estado) (Ingesta, error) {
	actual, err := r.git("rev-parse", "--verify", "refs/heads/"+e.Rama+"^{commit}")
	if err != nil {
		return Ingesta{}, fmt.Errorf("la rama %s no existe en el repo principal: %w", e.Rama, err)
	}
	if len(e.Equipo) > 0 || !existe(e.Worktree) {
		return Ingesta{Punta: actual}, nil // con equipo, la rama del slice solo la mueve juntar
	}
	return r.puntaDe(e, e.Worktree, e.Rama, "slice")
}

// Punta devuelve el commit actual de la rama del slice en el repo principal (sin traer nada).
func (r Repo) Punta(e *Estado) (string, error) {
	return r.git("rev-parse", "--verify", "refs/heads/"+e.Rama+"^{commit}")
}

// tocadas lista los archivos que cambian entre dos commits del repo principal.
func (r Repo) tocadas(desde, hasta string) ([]string, error) {
	out, err := r.git("diff", "--name-only", "--no-renames", "-z", desde, hasta)
	if err != nil {
		return nil, err
	}
	var fs []string
	for _, f := range strings.Split(out, "\x00") {
		if f != "" {
			fs = append(fs, f)
		}
	}
	return fs, nil
}

// protegidasTocadas separa, de lo cambiado entre dos commits, lo que toca el examen y lo que toca
// otras rutas protegidas que el alcance del slice no autoriza.
func (r Repo) protegidasTocadas(e *Estado, desde, hasta string) (examen, otras []string, err error) {
	fs, err := r.tocadas(desde, hasta)
	if err != nil {
		return nil, nil, err
	}
	pats := r.protegidos()
	for _, f := range fs {
		if slices.Contains(e.Bundle, f) {
			examen = append(examen, f)
			continue
		}
		prot := false
		for _, p := range pats {
			prot = prot || coincide(f, p)
		}
		if !prot {
			continue
		}
		aut := false
		for _, a := range e.Alcance {
			aut = aut || dentro(a, f)
		}
		if !aut {
			otras = append(otras, f)
		}
	}
	return examen, otras, nil
}

// avanzarClon lleva un clon limpio a la punta del slice, dentro de la caja. Devuelve los archivos sin
// commit si los hay (y entonces no lo toca).
func (r Repo) avanzarClon(e *Estado, clon, punta string) (sucio string, err error) {
	if punta == e.SHARojo {
		return r.sucioDe(clon)
	}
	fuera, err := os.MkdirTemp("", "orq-avance-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(fuera)
	bundle := filepath.Join(fuera, "s.bundle")
	if _, err := r.git("bundle", "create", "-q", bundle, "refs/heads/"+e.Rama, "^"+e.SHARojo); err != nil {
		return "", err
	}
	const guion = `s=$(git status --porcelain 2>/dev/null); if [ -n "$s" ]; then printf '%s\n' "$s"; exit 3; fi
git fetch -q "$1" "refs/heads/$2" && git merge --ff-only -q FETCH_HEAD; exit 0`
	out, code, _, err := r.enCaja(sandbox.Spec{RW: []string{clon}, RO: []string{fuera}, Cwd: clon, Env: entornoExamen()},
		LimiteIngesta, nil, "sh", "-c", guion, "sh", bundle, e.Rama)
	if err != nil {
		return "", err
	}
	if code == 3 {
		return strings.TrimSpace(out), nil
	}
	return "", nil
}

// sucioDe pregunta al git del clon, dentro de la caja, qué tiene sin commit. Es un aviso.
func (r Repo) sucioDe(clon string) (string, error) {
	out, _, _, err := r.enCaja(sandbox.Spec{RO: []string{clon}, Cwd: clon, Env: entornoExamen()}, LimiteIngesta, nil, "git", "status", "--porcelain")
	return strings.TrimSpace(out), err
}
