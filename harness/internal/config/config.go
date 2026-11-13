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
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// Agente es con qué corre un rol o un worker.
type Agente struct {
	Kind     string `json:"agente,omitempty"` // claude | codex
	Modelo   string `json:"modelo,omitempty"`
	Esfuerzo string `json:"esfuerzo,omitempty"`
}

// Recursos son las reglas con que orq decide cuántos agentes caben en la máquina.
type Recursos struct {
	RAMPorAgenteGB float64 `json:"ram_por_agente_gb"` // memoria que se presupuesta por agente
	ReservaRAMGB   float64 `json:"reserva_ram_gb"`    // memoria disponible que nunca se entrega
	CPUPorAgente   float64 `json:"cpu_por_agente"`    // núcleos que se presupuestan por agente
	ReservaCPUs    float64 `json:"reserva_cpus"`      // núcleos que nunca se entregan
	CargaMax       float64 `json:"carga_max"`         // carga de 1 min por núcleo a partir de la cual no se lanza
}

// Config es la configuración efectiva.
type Config struct {
	Terminal string                       `json:"terminal"` // herdr | orca
	Roles    map[string]Agente            `json:"roles"`
	Workers  map[string]map[string]Agente `json:"workers"` // dificultad → agente → modelo y esfuerzo
	Recursos Recursos                     `json:"recursos"`

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
	Kinds = []string{"claude", "codex"}
)

// OrigenDefecto marca un valor que ningún archivo cambió.
const OrigenDefecto = "defecto"

// Defecto es el comportamiento de orq sin ningún archivo: las tablas que antes vivían en el código.
func Defecto() Config {
	return Config{
		Terminal: "herdr",
		Roles: map[string]Agente{
			"atena":        {"claude", "opus", "medium"},
			"orquestador":  {"claude", "sonnet", "medium"},
			"planificador": {"claude", "opus", "high"},
			"astra":        {"codex", "gpt-6-astra", "high"},
			"verificador":  {"claude", "sonnet", "medium"},
			"scout":        {"claude", "haiku", "medium"},
			"escriba":      {"claude", "haiku", ""},
		},
		Workers: map[string]map[string]Agente{
			"baja":  {"claude": {Modelo: "haiku", Esfuerzo: "medium"}, "codex": {Modelo: "gpt-6-luna", Esfuerzo: "medium"}},
			"media": {"claude": {Modelo: "sonnet", Esfuerzo: "medium"}, "codex": {Modelo: "gpt-6.1-sol", Esfuerzo: "medium"}},
			"alta":  {"claude": {Modelo: "opus", Esfuerzo: "high"}, "codex": {Modelo: "gpt-6.1-sol", Esfuerzo: "high"}},
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
	return c, nil
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
		if !slices.Contains(Kinds, a.Kind) {
			return fmt.Errorf("roles.%s.agente %q: usa %s", rol, a.Kind, strings.Join(Kinds, "|"))
		}
		if strings.TrimSpace(a.Modelo) == "" {
			return fmt.Errorf("roles.%s.modelo vacío", rol)
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
			if !slices.Contains(Kinds, kind) {
				return fmt.Errorf("workers.%s.%s: agente desconocido; usa %s", dif, kind, strings.Join(Kinds, "|"))
			}
			if a.Kind != "" && a.Kind != kind {
				return fmt.Errorf("workers.%s.%s.agente no se escribe: el agente es la clave", dif, kind)
			}
			if strings.TrimSpace(a.Modelo) == "" || strings.TrimSpace(a.Esfuerzo) == "" {
				return fmt.Errorf("workers.%s.%s: faltan modelo o esfuerzo", dif, kind)
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
	if a.Kind != d.Kind && a.Modelo == "" {
		a.Modelo = c.Workers["alta"][a.Kind].Modelo
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
