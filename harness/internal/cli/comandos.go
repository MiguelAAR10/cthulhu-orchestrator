package cli

// comandos es la tabla de lo que orq despacha: cada comando con sus subcomandos (nil si no tiene).
// No cambia ningún comportamiento: la usan las pruebas para que guías y skills solo nombren lo que existe.
var comandos = map[string][]string{
	"gate": nil, "watch": nil, "status": nil, "halt": nil, "init": nil, "check": nil, "fase": nil,
	"contexto": nil, "doc": nil, "doctor": nil, "up": nil, "relevo": nil, "alinear": nil, "metricas": nil,
	"pausar": nil, "reanudar": nil, "version": nil, "--skill": nil,
	"update":     nil,                   // orq2 only; under orq it stays an unknown command
	"pendientes": nil, "despachar": nil, // orq2 only; under orq they stay unknown commands
	"historia": {"abrir", "pausar", "reanudar"},
	"config":   {"ver"},
	"skills":   {"fijar", "verificar"},
	"msg":      {"send", "reconciliar", "leer"},
	"mem":      {"add", "ver", "compactar"},
	"slice": {"abrir", "tomar", "alcance", "verificar", "ruta", "rehuella", "reabrir", "reintentar", "cerrar",
		"lanzar", "examen", "podar", "juntar", "contrato", "decidir", "tarjeta", "aprobar", "recibo", "estado"},
	"sesion":   {"ver", "abrir", "cerrar", "reenviar"},
	"plan":     {"cargar", "ver", "hecha", "soltar", "revisar", "debatido"},
	"aprender": {"ver", "add", "confirmar", "refutar", "promover"},
	"sandbox":  {"selftest", "exec"},
	"skill":    {"eval", "notas", "comparar", "atena", "orquestador", "planificador", "astra", "scout", "worker", "verificador"},
}

// removidos son flags que ya no existen: ninguna guía ni skill de orq2 los ofrece.
var removidos = []string{"--en-sitio", "--aislado"}
