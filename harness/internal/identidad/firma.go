package identidad

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
)

// Registros decisivos en .orq: el mismo HMAC que las anclas. El proceso orq lee el token del archivo,
// así que un worker sin leer el token no puede fabricar una firma; quien lo lea sí (ver identity.md).

// Alcance liga una firma a este proyecto: una línea firmada copiada de otro .orq no vale aquí.
func Alcance(orqDir string) string {
	abs, err := filepath.Abs(orqDir)
	if err != nil {
		abs = orqDir
	}
	if r, err := filepath.EvalSymlinks(abs); err == nil {
		return r
	}
	return abs
}

func macRegistro(tok, clase, alcance string, v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	m := hmac.New(sha256.New, []byte(tok))
	fmt.Fprintf(m, "orq.%s\n%s\n", clase, alcance)
	m.Write(raw)
	return m.Sum(nil), nil
}

// FirmarRegistro firma v (con su campo de firma vacío). "" si no hay token.
// Quien llama decide antes si la acción está autorizada: esto no comprueba al llamador.
func FirmarRegistro(clase, alcance string, v any) string {
	tok := Leer()
	if tok == "" {
		return ""
	}
	mac, err := macRegistro(tok, clase, alcance, v)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(mac)
}

// RegistroValido comprueba la firma. Sin token, sin firma o con firma mala: false (falla cerrado).
func RegistroValido(clase, alcance string, v any, firma string) bool {
	tok := Leer()
	got, err := hex.DecodeString(firma)
	if tok == "" || firma == "" || err != nil {
		return false
	}
	mac, err := macRegistro(tok, clase, alcance, v)
	return err == nil && hmac.Equal(got, mac)
}
