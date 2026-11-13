package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Sonda es una comprobación de la autoprueba.
type Sonda struct {
	Nombre  string
	OK      bool
	Detalle string
}

var (
	unaVez     sync.Once
	errDispon  error
	tiempoCaja = 30 * time.Second
)

// Disponible exige bwrap y que la autoprueba pase entera. Se calcula una vez por proceso.
func Disponible() error {
	unaVez.Do(func() { errDispon = disponible() })
	return errDispon
}

func disponible() error {
	ss, err := Sondas()
	if err != nil {
		return err
	}
	var malas []string
	for _, s := range ss {
		if !s.OK {
			malas = append(malas, s.Nombre)
		}
	}
	if len(malas) > 0 {
		return fmt.Errorf("sandbox: la autoprueba falla (%s); corre `orq sandbox selftest`", strings.Join(malas, "; "))
	}
	return nil
}

// guion corre dentro de la caja. Argumentos: rw, ro, base, oculto, pid de fuera, home, nombre de escape.
const guion = `
p() { n=$1; shift; if "$@" >/dev/null 2>&1; then echo "$n|ok"; else echo "$n|fallo"; fi; }
no() { ! "$@"; }
p rw sh -c 'printf orq > "$1/ok" && test "$(cat "$1/ok")" = orq' _ "$1"
p ro no sh -c 'printf x > "$1/f"' _ "$2"
p raiz no sh -c 'printf x > /escape'
sh -c 'printf x > "$1/$2"' _ "$3" "$7" 2>/dev/null
sh -c 'printf x > "$1/$2"' _ "$6" "$7" 2>/dev/null
p oculto no test -e "$4"
p mnt no test -e /mnt
p config no test -e "$6/.config"
p herdr no test -e "$6/.config/herdr/herdr.sock"
p docker sh -c '! test -e /var/run/docker.sock && ! test -e /run/docker.sock'
p redif sh -c '! grep -v "^ *lo:" /proc/net/dev | grep -q ":"'
p redtcp no timeout 3 bash -c 'exec 3<>/dev/tcp/1.1.1.1/80'
p pid sh -c '! kill -0 "$1" && ! test -e "/proc/$1"' _ "$5"
p sudo no sudo -n true
p marca test -c /run/orq/sandbox
printf 'environ|'; tr '\0' ' ' < /proc/1/environ; echo
`

// Sondas corre la autoprueba completa y devuelve cada comprobación. El error es de preparación
// (sin bwrap, sin directorio temporal); una sonda fallida no es error, es OK=false.
func Sondas() ([]Sonda, error) {
	if _, err := buscarBwrap(); err != nil {
		return nil, err
	}
	base, err := os.MkdirTemp("", "orq-sandbox-autoprueba-")
	if err != nil {
		return nil, fmt.Errorf("sandbox: autoprueba: %w", err)
	}
	defer os.RemoveAll(base)
	rw, ro, oculto := filepath.Join(base, "rw"), filepath.Join(base, "ro"), filepath.Join(base, "oculto")
	for _, d := range []string{rw, ro, oculto} {
		if err := os.Mkdir(d, 0o755); err != nil {
			return nil, fmt.Errorf("sandbox: autoprueba: %w", err)
		}
	}
	const original = "intacto"
	if err := os.WriteFile(filepath.Join(ro, "f"), []byte(original), 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(oculto, "secreto"), []byte("s"), 0o644); err != nil {
		return nil, err
	}
	h, err := hostFn()
	if err != nil {
		return nil, err
	}
	escape := "orq-escape-" + filepath.Base(base)
	pid := strconv.Itoa(os.Getpid())

	ctx, cancel := context.WithTimeout(context.Background(), tiempoCaja)
	defer cancel()
	cmd, err := comando(ctx, Spec{RO: []string{ro}, RW: []string{rw}, Cwd: rw, Env: map[string]string{"ORQ_SONDA": "1"}},
		[]string{"sh", "-c", guion, "sh", rw, ro, base, filepath.Join(oculto, "secreto"), pid, h.Home, escape})
	if err != nil {
		return nil, err
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	runErr := cmd.Run()

	res := map[string]string{}
	for _, l := range strings.Split(out.String(), "\n") {
		if k, v, ok := strings.Cut(l, "|"); ok {
			res[k] = strings.TrimSpace(v)
		}
	}
	fallo := ""
	if runErr != nil && len(res) == 0 {
		fallo = fmt.Sprintf("la caja no arrancó: %v %s", runErr, strings.TrimSpace(errb.String()))
	}
	sonda := func(nombre, detalle string, ok bool) Sonda {
		if fallo != "" {
			return Sonda{Nombre: nombre, Detalle: fallo}
		}
		return Sonda{Nombre: nombre, OK: ok, Detalle: detalle}
	}
	is := func(k string) bool { return res[k] == "ok" }

	// Lo que de verdad importa de una escritura es el host: se mira desde fuera.
	sigue := false
	if raw, err := os.ReadFile(filepath.Join(ro, "f")); err == nil && string(raw) == original {
		sigue = true
	}
	escaparon := []string{}
	for _, p := range []string{filepath.Join(base, escape), filepath.Join(h.Home, escape), filepath.Join(ro, escape)} {
		if _, err := os.Lstat(p); err == nil {
			escaparon = append(escaparon, p)
			os.Remove(p)
		}
	}

	envOK, envDet := true, ""
	permitidas := map[string]bool{"PATH": true, "HOME": true, "ORQ_SONDA": true}
	for _, kv := range strings.Fields(res["environ"]) {
		k, _, _ := strings.Cut(kv, "=")
		if !permitidas[k] {
			envOK = false
			envDet += " " + k
		}
	}
	if _, ok := res["environ"]; !ok {
		envOK, envDet = false, " sin lectura de /proc/1/environ"
	}

	return []Sonda{
		sonda("escribir en RW funciona", rw, is("rw")),
		sonda("escribir en RO falla", ro, is("ro") && sigue),
		sonda("escribir fuera de RW no llega al host", "raíz de solo lectura; ningún archivo de escape en el host "+strings.Join(escaparon, " "),
			is("raiz") && len(escaparon) == 0),
		sonda("ruta no permitida ausente", "otro directorio del host, /mnt y ~/.config", is("oculto") && is("mnt") && is("config")),
		sonda("socket de herdr ausente", "~/.config/herdr/herdr.sock", is("herdr")),
		sonda("socket de docker ausente", "/var/run/docker.sock", is("docker")),
		sonda("red inalcanzable", "solo lo, y TCP a 1.1.1.1:80 falla", is("redif") && is("redtcp")),
		sonda("pid de fuera invisible", "pid "+pid, is("pid")),
		sonda("sudo -n true falla", "no new privileges", is("sudo")),
		sonda("marca "+Marca+" presente", "", is("marca")),
		sonda("entorno de pid 1 limpio", "solo PATH, HOME y lo pedido"+envDet, envOK),
	}, nil
}
