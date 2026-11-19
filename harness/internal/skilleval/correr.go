package skilleval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Caso es la nota de un caso: la media de sus corridas.
type Caso struct {
	Nombre string  `json:"nombre"`
	Nota   float64 `json:"nota"`
}

// Resultado es lo que orq usa de aggregate-result.json.
type Resultado struct {
	Nota     float64
	Casos    []Caso
	Corridas int
	CostoUSD float64
	Claude   string
	Parcial  bool
	Fallidas int    // corridas con error o abortadas
	Error    string // el primer error de una corrida
}

// Leer interpreta aggregate-result.json (schemaVersion 1).
func Leer(raw []byte) (Resultado, error) {
	var a struct {
		SchemaVersion int     `json:"schemaVersion"`
		ClaudeVersion string  `json:"claudeVersion"`
		CostUSD       float64 `json:"costUsd"`
		Partial       bool    `json:"partial"`
		Aggregates    struct {
			OverallScore float64 `json:"overallScore"`
		} `json:"aggregates"`
		Cases []struct {
			Name       string `json:"name"`
			Aggregates struct {
				Score float64 `json:"score"`
			} `json:"aggregates"`
			Arms struct {
				With []struct {
					Error   string `json:"error"`
					Aborted bool   `json:"aborted"`
				} `json:"with"`
			} `json:"arms"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return Resultado{}, fmt.Errorf("aggregate-result.json: %w", err)
	}
	if a.SchemaVersion != 1 {
		return Resultado{}, fmt.Errorf("aggregate-result.json: schemaVersion %d, orq lee la 1", a.SchemaVersion)
	}
	r := Resultado{Nota: a.Aggregates.OverallScore, CostoUSD: a.CostUSD, Claude: a.ClaudeVersion, Parcial: a.Partial}
	for _, c := range a.Cases {
		r.Casos = append(r.Casos, Caso{Nombre: c.Name, Nota: c.Aggregates.Score})
		r.Corridas += len(c.Arms.With)
		for _, w := range c.Arms.With {
			if w.Error == "" && !w.Aborted {
				continue
			}
			r.Fallidas++
			if r.Error == "" {
				r.Error = w.Error
				if r.Error == "" {
					r.Error = "corrida abortada"
				}
			}
		}
	}
	// Una corrida con error o abortada no puntuó: la nota no cubre todo lo pedido.
	r.Parcial = r.Parcial || r.Fallidas > 0
	return r, nil
}

// ErrNoDisponible: la cuenta no tiene `claude plugin eval` (acceso anticipado) o no hay claude.
var ErrNoDisponible = errors.New("claude plugin eval no está disponible en esta cuenta (acceso anticipado); no se evalúa")

var noDisponible = regexp.MustCompile(`(?i)early access|not available|unavailable|unknown command`)

// Correr evalúa el plugin con `claude plugin eval`, con tope de gasto maxUSD, y lee su resultado.
// Salida 1 (un caso bajo el umbral) es una nota; salida 2 (tope de costo) es una nota parcial.
func Correr(ctx context.Context, plugin, jsonPath string, maxUSD float64) (Resultado, error) {
	cmd := exec.CommandContext(ctx, "claude", "plugin", "eval", plugin, "--json", jsonPath,
		"--ablation", "none", "--max-cost-usd", strconv.FormatFloat(maxUSD, 'f', -1, 64), "--trust-plugin")
	var errb bytes.Buffer
	cmd.Stderr = &errb
	errC := cmd.Run()
	if errors.Is(errC, exec.ErrNotFound) {
		return Resultado{}, fmt.Errorf("%w: claude no está en PATH", ErrNoDisponible)
	}
	if cmd.ProcessState == nil {
		return Resultado{}, fmt.Errorf("claude plugin eval no arrancó: %w", errC)
	}
	salida := cmd.ProcessState.ExitCode()
	if raw, err := os.ReadFile(jsonPath); err == nil && salida >= 0 && salida <= 2 {
		r, err := Leer(raw)
		r.Parcial = r.Parcial || salida == 2
		if err == nil && r.Corridas > 0 && r.Fallidas == r.Corridas {
			return Resultado{}, fmt.Errorf("las %d corridas fallaron, no se guarda nota: %s", r.Corridas, r.Error)
		}
		return r, err
	}
	msg := strings.TrimSpace(errb.String())
	if noDisponible.MatchString(msg) {
		primera, _, _ := strings.Cut(msg, "\n")
		return Resultado{}, fmt.Errorf("%w: %s", ErrNoDisponible, primera)
	}
	if len(msg) > 500 {
		msg = msg[len(msg)-500:]
	}
	return Resultado{}, fmt.Errorf("claude plugin eval salió con %d sin resultado: %v %s", salida, errC, msg)
}
