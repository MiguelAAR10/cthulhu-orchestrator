package sandbox

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

// Red is the network a box gets besides Spec.Net (the host network, used by the exam's --red local).
type Red int

const (
	// SinRed: no network at all.
	SinRed Red = iota
	// RedProxy: the box has its own netns; its only way out is a unix socket from Spec.Proxy,
	// mounted at DirRed, served outside by Proxy.
	RedProxy
)

// DirRed is where the outside socket dir is mounted inside the box; the socket is DirRed/p.sock.
const DirRed = "/run/orq/red"

// SocketRed is the proxy socket's file name inside the socket dir.
const SocketRed = "p.sock"

// DirSocket makes a private dir for the proxy socket under /tmp, never under TMPDIR or the state dir:
// a unix socket path must fit in 108 bytes, and a long state path did not ("AF_UNIX path too long").
func DirSocket() (string, error) {
	return os.MkdirTemp("/tmp", "orq-r")
}

// Proxy serves an allowlisting CONNECT proxy on ln until ln is closed. Only CONNECT to a host:port in
// permitidos is tunneled; an entry without a port means port 443. Every request writes one log line,
// "ALLOW host:port" or "DENY host:port" (never headers). The proxy resolves names itself, outside.
func Proxy(ln net.Listener, permitidos []string, log io.Writer) error {
	permitido := map[string]bool{}
	for _, p := range permitidos {
		if _, _, err := net.SplitHostPort(p); err != nil {
			p = net.JoinHostPort(p, "443")
		}
		permitido[p] = true
	}
	var mu sync.Mutex
	registrar := func(veredicto, destino string) {
		mu.Lock()
		defer mu.Unlock()
		fmt.Fprintf(log, "%s %s %s\n", time.Now().UTC().Format(time.RFC3339), veredicto, destino)
	}
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		c, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			atender(c, permitido, registrar)
		}()
	}
}

func atender(c net.Conn, permitido map[string]bool, registrar func(string, string)) {
	defer c.Close()
	c.SetReadDeadline(time.Now().Add(30 * time.Second))
	br := bufio.NewReader(c)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}
	c.SetReadDeadline(time.Time{})
	responder := func(code int) {
		fmt.Fprintf(c, "HTTP/1.1 %d %s\r\nContent-Length: 0\r\nConnection: close\r\n\r\n", code, http.StatusText(code))
	}
	destino := req.Host
	if req.Method != http.MethodConnect {
		registrar("DENY", req.Method+" "+destino)
		responder(http.StatusMethodNotAllowed)
		return
	}
	if !permitido[destino] {
		registrar("DENY", destino)
		responder(http.StatusForbidden)
		return
	}
	up, err := net.DialTimeout("tcp", destino, 15*time.Second)
	if err != nil {
		registrar("FAIL", destino)
		responder(http.StatusBadGateway)
		return
	}
	defer up.Close()
	registrar("ALLOW", destino)
	if _, err := io.WriteString(c, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	// Bytes the client sent after the request line are already in br.
	tunel(c, br, up)
}

// tunel copies both ways until either side ends, then closes both.
func tunel(c net.Conn, desdeC io.Reader, up net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(up, desdeC)
		cerrarEscritura(up)
	}()
	go func() {
		defer wg.Done()
		io.Copy(c, up)
		cerrarEscritura(c)
	}()
	wg.Wait()
}

func cerrarEscritura(c net.Conn) {
	if cw, ok := c.(interface{ CloseWrite() error }); ok {
		cw.CloseWrite()
		return
	}
	c.Close()
}

// Puente listens on addr (inside the box, 127.0.0.1:3128) and pipes every connection to the unix
// socket sock. It serves in the background until the returned listener is closed.
func Puente(sock, addr string) (net.Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("sandbox: proxy bridge: %w", err)
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				up, err := net.Dial("unix", sock)
				if err != nil {
					return
				}
				defer up.Close()
				tunel(c, c, up)
			}()
		}
	}()
	return ln, nil
}

// validarRed checks the network fields of a Spec and returns the socket dir mount, if any.
func validarRed(s Spec, h host) ([]montaje, error) {
	switch s.Red {
	case SinRed:
		if s.Proxy != "" {
			return nil, errors.New("sandbox: Proxy without RedProxy")
		}
		return nil, nil
	case RedProxy:
	default:
		return nil, fmt.Errorf("sandbox: unknown network %d", s.Red)
	}
	if s.Net {
		return nil, errors.New("sandbox: RedProxy and Net together: the proxy box never shares the host network")
	}
	if s.Proxy == "" {
		return nil, errors.New("sandbox: RedProxy needs the socket dir (Spec.Proxy)")
	}
	m, err := validar(s.Proxy, true, h)
	if err != nil {
		return nil, err
	}
	m.dst = DirRed
	return []montaje{m}, nil
}
