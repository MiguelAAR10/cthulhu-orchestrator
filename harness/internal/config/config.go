// Package config decide lo que antes eran constantes: qué terminal maneja las sesiones, qué agente,
// modelo y esfuerzo usa cada rol, qué modelo recibe un worker según la dificultad de su tarea y
// cuánta máquina se reserva por agente.
//
// Se leen tres capas, en este orden, y cada campo guarda de dónde salió:
//
//	valores por defecto  →  archivo del usuario  →  archivo del proyecto (<raíz>/.orq/config.json)
//
// Un campo que el programa no conoce es un error: una configuración mal escrita no se adivina.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// Agente es con qué corre un rol o un worker.
type Agente struct {
	Kind     string `json:"agente,omitempty"` // claude
	Modelo   string `json:"modelo,omitempty"`
	Esfuerzo string `json:"esfuerzo,omitempty"`
	// Entorno son variables de cuenta que el lanzador pone en el panel y que también usa la llamada
	// sin sesión del escriba: CLAUDE_CONFIG_DIR, CODEX_HOME. Sus valores son rutas, nunca secretos.
	Entorno map[string]string `json:"entorno,omitempty"`
}

// Recursos son las reglas con que orq decide cuántos agentes caben en la máquina.
type Recursos struct {
	RAMPorAgenteGB float64 `json:"ram_por_agente_gb"` // memoria que se presupuesta por agente
	ReservaRAMGB   float64 `json:"reserva_ram_gb"`    // memoria disponible que nunca se entrega
	CPUPorAgente   float64 `json:"cpu_por_agente"`    // núcleos que se presupuestan por agente
	ReservaCPUs    float64 `json:"reserva_cpus"`      // núcleos que nunca se entregan
	CargaMax       float64 `json:"carga_max"`         // carga de 1 min por núcleo a partir de la cual no se lanza
}

// Check es un comando del proyecto que decide si el código es correcto (lint, tipos, pruebas o una
// guardia extra). Lleva el comando o, si el proyecto no lo tiene, por qué se omite: omitido no es pasado.
type Check struct {
	Nombre string `json:"-"`
	Cmd    string `json:"cmd,omitempty"`
	Omitir string `json:"omitir,omitempty"`
	// Patrones solo en checks.supresiones: marcas de supresión que se suman a las de SupresionesBase.
	Patrones []string `json:"patrones,omitempty"`
	// Archivos solo en checks.definiciones: más archivos que definen lo que corre un check.
	Archivos []string `json:"archivos,omitempty"`
	// Servidor y Puerto solo en checks.e2e: el servidor de la app que se arranca antes de Cmd y el
	// puerto de 127.0.0.1 en que se espera que escuche.
	Servidor string `json:"servidor,omitempty"`
	Puerto   int    `json:"puerto,omitempty"`
	// EsperaS, solo con Servidor: segundos que se espera a que abra el puerto (0 = EsperaDefecto).
	EsperaS int `json:"espera_s,omitempty"`
}

// EsperaDefecto y EsperaMax acotan, en segundos, la espera del servidor de checks.e2e.
const (
	EsperaDefecto = 60
	EsperaMax     = 600
)

// Nombres reservados del bloque checks. supresiones es una guardia de orq, encendida siempre que hay
// bloque checks; se apaga con omitir. definiciones no corre: lista archivos que deciden qué corre un check.
const (
	Supresiones  = "supresiones"
	Definiciones = "definiciones"
	E2E          = "e2e"
)

// SupresionesBase son las marcas que la guardia supresiones busca en las líneas que suma un slice.
// Las pruebas enfocadas (.only, fit, fdescribe) también lo son: dejan fuera al resto de la suite.
var SupresionesBase = []string{"//nolint", "eslint-disable", "@ts-ignore", "@ts-expect-error", "# type: ignore", "# noqa", "#[allow(",
	".only(", "describe.only(", "fit(", "fdescribe("}

// Interno dice si el check lo corre orq y no un comando del proyecto.
func (ch Check) Interno() bool { return ch.Nombre == Supresiones }

// Marcas son las marcas de supresión que busca la guardia: las de base y las del proyecto.
func (ch Check) Marcas() []string { return append(slices.Clone(SupresionesBase), ch.Patrones...) }

// ArchivosDefinicion son los archivos que el proyecto suma a los que definen sus checks.
func (c Config) ArchivosDefinicion() []string { return c.Checks[Definiciones].Archivos }

// ChecksBase son los que todo bloque checks declara; cualquier otro nombre es una guardia extra.
var ChecksBase = []string{"lint", "types", "tests"}

var nombreCheck = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// ChecksEnOrden devuelve lint, types y tests, y después las guardias extra por nombre, supresiones
// incluida aunque el bloque no la nombre. Sin bloque checks no devuelve ninguno.
func (c Config) ChecksEnOrden() []Check {
	var extra []string
	for n := range c.Checks {
		if !slices.Contains(ChecksBase, n) && n != Definiciones {
			extra = append(extra, n)
		}
	}
	if _, ok := c.Checks[Supresiones]; len(c.Checks) > 0 && !ok {
		extra = append(extra, Supresiones)
	}
	sort.Strings(extra)
	var out []Check
	for _, n := range append(slices.Clone(ChecksBase), extra...) {
		if ch, ok := c.Checks[n]; ok || n == Supresiones {
			ch.Nombre = n
			out = append(out, ch)
		}
	}
	return out
}

// ValidarKind acepta solo los agentes que orq arranca. Codex no se usa nunca, en ningún rol.
func ValidarKind(kind string) error {
	switch {
	case kind == "codex":
		return errors.New(`agente "codex": Codex no se usa en ningún rol ni worker (regla del Owner, 2026-11-18); usa claude`)
	case !slices.Contains(Kinds, kind):
		return fmt.Errorf("agente %q: usa %s", kind, strings.Join(Kinds, "|"))
	}
	return nil
}

// Config es la configuración efectiva.
type Config struct {
	Terminal string                       `json:"terminal"` // herdr | orca
	Roles    map[string]Agente            `json:"roles"`
	Workers  map[string]map[string]Agente `json:"workers"` // dificultad → agente → modelo y esfuerzo
	Recursos Recursos                     `json:"recursos"`
	Checks   map[string]Check             `json:"checks,omitempty"` // nombre → comando; ver ChecksEnOrden

	origen   map[string]string // clave con puntos → de dónde salió
	archivos []Archivo
}

// Archivo es una capa leída (o que se habría leído).
type Archivo struct {
	Ruta   string
	Existe bool
}

var (
	// Terminales son los drivers de terminal que orq sabe manejar.
	Terminales = []string{"herdr", "orca"}
	// Roles con agente configurable. «escriba» es el modelo barato que resume memoria y relevos.
	Roles = []string{"atena", "orquestador", "planificador", "astra", "verificador", "scout", "escriba"}
	// Dificultades de una tarea de worker.
	Dificultades = []string{"baja", "media", "alta"}
	// Kinds son las CLI de agente que orq sabe arrancar.
	Kinds = []string{"claude"}
)

// OrigenDefecto marca un valor que ningún archivo cambió.
const OrigenDefecto = "defecto"

// Defecto es el comportamiento de orq sin ningún archivo: las tablas que antes vivían en el código.
func Defecto() Config {
	return Config{
		Terminal: "herdr",
		Roles: map[string]Agente{
			"atena":        {Kind: "claude", Modelo: "opus", Esfuerzo: "medium"},
			"orquestador":  {Kind: "claude", Modelo: "sonnet", Esfuerzo: "medium"},
			"planificador": {Kind: "claude", Modelo: "opus", Esfuerzo: "high"},
			"astra":        {Kind: "claude", Modelo: "opus", Esfuerzo: "high"},
			"verificador":  {Kind: "claude", Modelo: "sonnet", Esfuerzo: "medium"},
			"scout":        {Kind: "claude", Modelo: "haiku", Esfuerzo: "medium"},
			"escriba":      {Kind: "claude", Modelo: "haiku"},
		},
		Workers: map[string]map[string]Agente{
			"baja":  {"claude": {Modelo: "haiku", Esfuerzo: "medium"}},
			"media": {"claude": {Modelo: "sonnet", Esfuerzo: "medium"}},
			"alta":  {"claude": {Modelo: "opus", Esfuerzo: "high"}},
		},
		Recursos: Recursos{RAMPorAgenteGB: 1.5, ReservaRAMGB: 2, CPUPorAgente: 1, ReservaCPUs: 1, CargaMax: 1.5},
	}
}

// RutaUsuario respeta ORQ_CONFIG (pruebas) y usa ~/.config/orq/config.json por defecto.
func RutaUsuario() string {
	if p := os.Getenv("ORQ_CONFIG"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "orq", "config.json")
}

// RutaProyecto es el archivo del proyecto; vive en .orq, que un worker no escribe.
func RutaProyecto(root string) string { return filepath.Join(root, ".orq", "config.json") }

// Cargar une defecto, usuario y proyecto. root vacío = sin capa de proyecto.
func Cargar(root string) (Config, error) {
	rutas := []string{RutaUsuario()}
	if root != "" {
		rutas = append(rutas, RutaProyecto(root))
	}
	return CargarDe(rutas...)
}

// CargarDe une el defecto con cada archivo, en orden; el último gana campo a campo.
func CargarDe(rutas ...string) (Config, error) {
	base, err := generico(Defecto())
	if err != nil {
		return Config{}, err
	}
	origen := map[string]string{}
	marcar(base, "", OrigenDefecto, origen)
	var archivos []Archivo
	for _, ruta := range rutas {
		raw, err := os.ReadFile(ruta)
		if errors.Is(err, fs.ErrNotExist) {
			archivos = append(archivos, Archivo{Ruta: ruta})
			continue
		}
		if err != nil {
			return Config{}, fmt.Errorf("config: no se pudo leer %s: %w", ruta, err)
		}
		archivos = append(archivos, Archivo{Ruta: ruta, Existe: true})
		// Primero contra el esquema: un campo desconocido es un error, no se ignora.
		var esquema Config
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&esquema); err != nil {
			return Config{}, fmt.Errorf("config: %s: %w", ruta, err)
		}
		if dec.More() {
			return Config{}, fmt.Errorf("config: %s: sobra contenido después del objeto", ruta)
		}
		var capa map[string]any
		if err := json.Unmarshal(raw, &capa); err != nil {
			return Config{}, fmt.Errorf("config: %s: %w", ruta, err)
		}
		unir(base, capa, "", ruta, origen)
	}
	raw, err := json.Marshal(base)
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	c.origen, c.archivos = origen, archivos
	if m := os.Getenv("ORQ_ESCRIBA_MODELO"); m != "" { // compatibilidad: la variable manda sobre los archivos
		a := c.Roles["escriba"]
		a.Modelo = m
		c.Roles["escriba"] = a
		c.origen["roles.escriba.modelo"] = "ORQ_ESCRIBA_MODELO"
	}
	if err := c.validar(); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	c.expandirEntornos()
	return c, nil
}

var (
	nombreVar   = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	pareceClave = regexp.MustCompile(`TOKEN|SECRET|PASSWORD|PASSWD|CREDENTIAL|API_?KEY|(^|_)KEY($|_)`)
)

// validarEntorno: solo variables que apuntan a un directorio de cuenta. Un secreto no vive en este archivo.
func validarEntorno(donde string, env map[string]string) error {
	for k, v := range env {
		switch {
		case !nombreVar.MatchString(k):
			return fmt.Errorf("%s.entorno.%s: nombre de variable inválido", donde, k)
		case pareceClave.MatchString(k):
			return fmt.Errorf("%s.entorno.%s: parece un secreto; aquí solo van rutas (CLAUDE_CONFIG_DIR, CODEX_HOME). Un secreto no se escribe en la configuración", donde, k)
		case k == "PATH" || k == "HOME" || strings.HasPrefix(k, "ORQ_") || strings.HasPrefix(k, "LD_"):
			return fmt.Errorf("%s.entorno.%s: esa variable la pone orq o el sistema", donde, k)
		case !strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "~/"):
			return fmt.Errorf("%s.entorno.%s: el valor debe ser una ruta absoluta o ~/…, no %q", donde, k, v)
		}
	}
	return nil
}

// expandirEntornos resuelve ~/ en las rutas de cuenta.
func (c *Config) expandirEntornos() {
	home, _ := os.UserHomeDir()
	exp := func(env map[string]string) {
		for k, v := range env {
			if strings.HasPrefix(v, "~/") && home != "" {
				env[k] = filepath.Join(home, v[2:])
			}
		}
	}
	for _, a := range c.Roles {
		exp(a.Entorno)
	}
	for _, porKind := range c.Workers {
		for _, a := range porKind {
			exp(a.Entorno)
		}
	}
}

func generico(v any) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	return m, json.Unmarshal(raw, &m)
}

// marcar anota el origen de cada hoja de m.
func marcar(m map[string]any, prefijo, de string, origen map[string]string) {
	for k, v := range m {
		clave := unirClave(prefijo, k)
		if hijo, ok := v.(map[string]any); ok {
			marcar(hijo, clave, de, origen)
			continue
		}
		origen[clave] = de
	}
}

// unir vuelca capa sobre base hoja por hoja: lo que la capa no nombra queda como estaba.
func unir(base, capa map[string]any, prefijo, de string, origen map[string]string) {
	for k, v := range capa {
		clave := unirClave(prefijo, k)
		hijo, esMapa := v.(map[string]any)
		if !esMapa {
			base[k] = v
			origen[clave] = de
			continue
		}
		destino, ok := base[k].(map[string]any)
		if !ok {
			destino = map[string]any{}
			base[k] = destino
		}
		unir(destino, hijo, clave, de, origen)
	}
}

func unirClave(prefijo, k string) string {
	if prefijo == "" {
		return k
	}
	return prefijo + "." + k
}

func (c Config) validar() error {
	if !slices.Contains(Terminales, c.Terminal) {
		return fmt.Errorf("terminal %q: usa %s", c.Terminal, strings.Join(Terminales, "|"))
	}
	for rol, a := range c.Roles {
		if !slices.Contains(Roles, rol) {
			return fmt.Errorf("roles.%s: rol desconocido; los roles son %s", rol, strings.Join(Roles, ", "))
		}
		if err := ValidarKind(a.Kind); err != nil {
			return fmt.Errorf("roles.%s: %w", rol, err)
		}
		if strings.TrimSpace(a.Modelo) == "" {
			return fmt.Errorf("roles.%s.modelo vacío", rol)
		}
		if err := validarEntorno("roles."+rol, a.Entorno); err != nil {
			return err
		}
	}
	for _, rol := range Roles {
		if _, ok := c.Roles[rol]; !ok {
			return fmt.Errorf("falta roles.%s", rol)
		}
	}
	for dif, porKind := range c.Workers {
		if !slices.Contains(Dificultades, dif) {
			return fmt.Errorf("workers.%s: dificultad desconocida; usa %s", dif, strings.Join(Dificultades, "|"))
		}
		for kind, a := range porKind {
			if err := ValidarKind(kind); err != nil {
				return fmt.Errorf("workers.%s.%s: %w", dif, kind, err)
			}
			if a.Kind != "" && a.Kind != kind {
				return fmt.Errorf("workers.%s.%s.agente no se escribe: el agente es la clave", dif, kind)
			}
			if strings.TrimSpace(a.Modelo) == "" || strings.TrimSpace(a.Esfuerzo) == "" {
				return fmt.Errorf("workers.%s.%s: faltan modelo o esfuerzo", dif, kind)
			}
			if err := validarEntorno("workers."+dif+"."+kind, a.Entorno); err != nil {
				return err
			}
		}
	}
	for _, dif := range Dificultades {
		for _, kind := range Kinds {
			if _, ok := c.Workers[dif][kind]; !ok {
				return fmt.Errorf("falta workers.%s.%s", dif, kind)
			}
		}
	}
	if err := c.validarChecks(); err != nil {
		return err
	}
	r := c.Recursos
	switch {
	case r.RAMPorAgenteGB <= 0:
		return errors.New("recursos.ram_por_agente_gb debe ser mayor que 0")
	case r.CPUPorAgente <= 0:
		return errors.New("recursos.cpu_por_agente debe ser mayor que 0")
	case r.ReservaRAMGB < 0, r.ReservaCPUs < 0:
		return errors.New("recursos: una reserva no puede ser negativa")
	case r.CargaMax <= 0:
		return errors.New("recursos.carga_max debe ser mayor que 0")
	}
	return nil
}

func (c Config) validarChecks() error {
	if len(c.Checks) == 0 {
		return nil
	}
	for _, n := range ChecksBase {
		if _, ok := c.Checks[n]; !ok {
			return fmt.Errorf("falta checks.%s: el bloque checks declara lint, types y tests (con cmd u omitir)", n)
		}
	}
	for n, ch := range c.Checks {
		if !nombreCheck.MatchString(n) {
			return fmt.Errorf("checks.%s: nombre inválido; usa minúsculas, cifras y guiones", n)
		}
		sinCmd, sinOmitir := strings.TrimSpace(ch.Cmd) == "", strings.TrimSpace(ch.Omitir) == ""
		switch {
		case n == Definiciones:
			if !sinCmd || !sinOmitir || len(ch.Patrones) > 0 || len(ch.Archivos) == 0 {
				return errors.New("checks.definiciones: lleva solo archivos, la lista de los que definen lo que corre un check")
			}
		case n == Supresiones:
			if !sinCmd || len(ch.Archivos) > 0 || (!sinOmitir && len(ch.Patrones) > 0) {
				return errors.New("checks.supresiones: la corre orq; lleva patrones para sumar marcas u omitir (el motivo) para apagarla")
			}
		case (ch.Servidor != "" || ch.Puerto != 0) && (n != E2E || ch.Servidor == "" || ch.Puerto < 1 || ch.Puerto > 65535 || sinCmd):
			return fmt.Errorf("checks.%s: servidor y puerto (1-65535) van juntos, solo en checks.e2e y con cmd", n)
		case ch.EsperaS != 0 && (ch.Servidor == "" || ch.EsperaS < 1 || ch.EsperaS > EsperaMax):
			return fmt.Errorf("checks.%s: espera_s (1-%d) va solo con servidor, en checks.e2e", n, EsperaMax)
		case len(ch.Patrones) > 0 || len(ch.Archivos) > 0:
			return fmt.Errorf("checks.%s: patrones va solo en checks.supresiones y archivos solo en checks.definiciones", n)
		case sinCmd == sinOmitir:
			return fmt.Errorf("checks.%s: lleva cmd u omitir (el motivo), uno de los dos", n)
		}
	}
	return nil
}

// Rol devuelve el agente configurado de un rol.
func (c Config) Rol(rol string) Agente { return c.Roles[rol] }

// ParaRol completa lo que el llamador no fijó (kind, modelo, esfuerzo vacíos) con lo configurado.
// Si se pide otro agente que el del rol y no se dice modelo, se usa el de ese agente para trabajo difícil.
func (c Config) ParaRol(rol string, pedido Agente) Agente {
	d := c.Roles[rol]
	a := pedido
	if a.Kind == "" {
		a.Kind = d.Kind
	}
	if a.Kind != d.Kind { // otro agente que el del rol: su modelo y su cuenta, no los del rol
		if a.Modelo == "" {
			a.Modelo = c.Workers["alta"][a.Kind].Modelo
		}
		if a.Entorno == nil {
			a.Entorno = c.Workers["alta"][a.Kind].Entorno
		}
	} else if a.Entorno == nil {
		a.Entorno = d.Entorno
	}
	if a.Modelo == "" {
		a.Modelo = d.Modelo
	}
	if a.Esfuerzo == "" {
		a.Esfuerzo = d.Esfuerzo
	}
	return a
}

// Worker traduce agente y dificultad a modelo y esfuerzo. Dificultad vacía = media.
func (c Config) Worker(kind, dificultad string) (modelo, esfuerzo string) {
	if dificultad == "" {
		dificultad = "media"
	}
	a := c.Workers[dificultad][kind]
	return a.Modelo, a.Esfuerzo
}

// WorkerEntorno son las variables de cuenta de un worker de ese agente y dificultad.
func (c Config) WorkerEntorno(kind, dificultad string) map[string]string {
	if dificultad == "" {
		dificultad = "media"
	}
	return c.Workers[dificultad][kind].Entorno
}

// Cuenta describe en una línea con qué cuenta corre un agente: sus variables, o la del entorno de quien lanza.
func Cuenta(env map[string]string) string {
	if len(env) == 0 {
		return "la del entorno de quien lanza"
	}
	ks := make([]string, 0, len(env))
	for k := range env {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	var xs []string
	for _, k := range ks {
		xs = append(xs, k+"="+env[k])
	}
	return strings.Join(xs, " ")
}

// Linea es un valor efectivo y su origen, para `orq config ver`.
type Linea struct{ Clave, Valor, Origen string }

// Lineas devuelve cada valor efectivo, ordenado por clave.
func (c Config) Lineas() []Linea {
	m, err := generico(c)
	if err != nil {
		return nil
	}
	var out []Linea
	var andar func(m map[string]any, prefijo string)
	andar = func(m map[string]any, prefijo string) {
		for k, v := range m {
			clave := unirClave(prefijo, k)
			if hijo, ok := v.(map[string]any); ok {
				andar(hijo, clave)
				continue
			}
			raw, _ := json.Marshal(v)
			de := c.origen[clave]
			if de == "" {
				de = OrigenDefecto
			}
			out = append(out, Linea{Clave: clave, Valor: strings.Trim(string(raw), `"`), Origen: de})
		}
	}
	andar(m, "")
	sort.Slice(out, func(i, j int) bool { return out[i].Clave < out[j].Clave })
	return out
}

// Origen dice de dónde salió una clave («roles.astra.modelo»).
func (c Config) Origen(clave string) string {
	if de := c.origen[clave]; de != "" {
		return de
	}
	return OrigenDefecto
}

// Archivos son las capas consultadas, existan o no.
func (c Config) Archivos() []Archivo { return c.archivos }

// TablaWorkers resume en una línea qué modelo recibe cada dificultad, para las guías.
func (c Config) TablaWorkers() string {
	var partes []string
	for _, kind := range Kinds {
		var xs []string
		for _, dif := range Dificultades {
			a := c.Workers[dif][kind]
			xs = append(xs, fmt.Sprintf("%s=%s/%s", dif, a.Modelo, a.Esfuerzo))
		}
		partes = append(partes, kind+" "+strings.Join(xs, ", "))
	}
	return strings.Join(partes, "; ")
}

// Expandir reemplaza en una guía las marcas {{rol:NOMBRE}} (modelo del rol) y {{workers}}.
func (c Config) Expandir(texto string) string {
	for _, rol := range Roles {
		texto = strings.ReplaceAll(texto, "{{rol:"+rol+"}}", c.Roles[rol].Modelo)
	}
	return strings.ReplaceAll(texto, "{{workers}}", c.TablaWorkers())
}

// servidorSh arranca el servidor de la app en su propio grupo de procesos, espera hasta espera_s
// (60 s por defecto) a que abra 127.0.0.1:puerto, corre el comando y al salir mata al grupo. Sale con el código del comando.
const servidorSh = `orq_log=$(mktemp)
setsid sh -c %[1]s >"$orq_log" 2>&1 </dev/null &
orq_srv=$!
trap 'kill -TERM -- -$orq_srv 2>/dev/null' EXIT
orq_n=0
until bash -c ': </dev/tcp/127.0.0.1/%[2]d' 2>/dev/null; do
	orq_n=$((orq_n+1))
	if [ $orq_n -gt %[4]d ] || ! kill -0 $orq_srv 2>/dev/null; then
		echo "orq: el servidor de checks.e2e no abrió 127.0.0.1:%[2]d en %[5]d s (checks.e2e.espera_s)"; tail -n 20 "$orq_log"; exit 1
	fi
	sleep 0.25
done
%[3]s
`

// Ejecutable es el script que corre el check: Cmd, o Cmd con el servidor de la app arrancado antes.
func (ch Check) Ejecutable() string { return ch.ConServidor(SinOnly(ch.Cmd)) }

// EjecutableEn es Ejecutable para el proyecto de dir: un script de npm/pnpm/yarn que es `playwright test`
// también recibe --forbid-only.
func (ch Check) EjecutableEn(dir string) string { return ch.ConServidor(SinOnlyEn(dir, ch.Cmd)) }

// pmScript es `npm run X`, `npm test`, `pnpm [run] X` o `yarn [run] X` con sus argumentos hasta el
// siguiente separador de shell.
var pmScript = regexp.MustCompile(`\b(npm|pnpm|yarn)(\s+(?:run|run-script))?\s+([A-Za-z0-9_:.@/-]+)([^;&|\n]*)`)

// scriptPlaywright es un script que es solo `playwright test` con flags: lo que se le sume llega a playwright.
var scriptPlaywright = regexp.MustCompile("^(?:npx\\s+)?playwright\\s+test\\b[^;&|<>`$()\\n]*$")

var separadorArgs = regexp.MustCompile(`(^|\s)--(\s|$)`)

// MaxPackageJSON acota lo que se lee de package.json al resolver scripts.
const MaxPackageJSON = 1 << 20

// SinOnlyEn es SinOnly más los scripts: `npm run test:e2e` cuyo script es `playwright test` pasa a
// `npm run test:e2e -- --forbid-only` (pnpm y yarn sin el `--`). Un script compuesto no se toca.
func SinOnlyEn(dir, cmd string) string {
	cmd = SinOnly(cmd)
	scripts := scriptsEn(dir)
	if len(scripts) == 0 {
		return cmd
	}
	return pmScript.ReplaceAllStringFunc(cmd, func(m string) string {
		g := pmScript.FindStringSubmatch(m)
		pm, run, nombre, resto := g[1], g[2], g[3], g[4]
		if pm == "npm" && run == "" && nombre != "test" && nombre != "t" {
			return m
		}
		if nombre == "t" {
			nombre = "test"
		}
		sc := strings.TrimSpace(scripts[nombre])
		if !scriptPlaywright.MatchString(sc) || strings.Contains(sc+resto, "--forbid-only") {
			return m
		}
		cuerpo := strings.TrimRight(m, " \t")
		cola := m[len(cuerpo):]
		if pm == "npm" && !separadorArgs.MatchString(resto) {
			return cuerpo + " -- --forbid-only" + cola
		}
		return cuerpo + " --forbid-only" + cola
	})
}

func scriptsEn(dir string) map[string]string {
	f, err := os.Open(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, MaxPackageJSON))
	if err != nil {
		return nil
	}
	var pj struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(raw, &pj) != nil {
		return nil
	}
	return pj.Scripts
}

var playwrightTest = regexp.MustCompile(`\bplaywright\s+test\b`)

// SinOnly suma --forbid-only a cada `playwright test` de cmd: un test.only nuevo haría que Playwright
// corriera solo ese y dejara fuera el examen. Los scripts de npm/pnpm/yarn los resuelve SinOnlyEn.
func SinOnly(cmd string) string {
	if strings.Contains(cmd, "--forbid-only") {
		return cmd
	}
	return playwrightTest.ReplaceAllString(cmd, "$0 --forbid-only")
}

// ConServidor envuelve cmd con el servidor del check, si lo tiene.
func (ch Check) ConServidor(cmd string) string {
	if ch.Servidor == "" {
		return cmd
	}
	espera := ch.EsperaS
	if espera <= 0 {
		espera = EsperaDefecto
	}
	return fmt.Sprintf(servidorSh, "'"+strings.ReplaceAll(ch.Servidor, "'", `'\''`)+"'", ch.Puerto, cmd, espera*4, espera)
}
