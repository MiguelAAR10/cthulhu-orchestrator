package escriba

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestAskCortaPorTimeout(t *testing.T) {
	orig := comando
	comando = func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "sleep", "5") }
	t.Cleanup(func() { comando = orig })
	t.Setenv("ORQ_ESCRIBA_TIMEOUT", "0.2")
	t0 := time.Now()
	_, err := Ask("hola")
	if err == nil || !strings.Contains(err.Error(), "tiempo") {
		t.Fatalf("debe fallar por tiempo: %v", err)
	}
	if time.Since(t0) > 3*time.Second {
		t.Fatalf("no respetó el timeout: %s", time.Since(t0))
	}
}

func TestTimeoutPorDefecto(t *testing.T) {
	t.Setenv("ORQ_ESCRIBA_TIMEOUT", "")
	if Timeout() != 120*time.Second {
		t.Fatalf("%s", Timeout())
	}
}
