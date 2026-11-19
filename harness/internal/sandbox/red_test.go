package sandbox

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuf is a log writer the proxy goroutines and the test can share.
type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// proxyEnSocket serves Proxy on a unix socket in a short temp dir and returns the socket path.
func proxyEnSocket(t *testing.T, permitidos []string) (string, *syncBuf) {
	t.Helper()
	dir, err := DirSocket()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "p.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	log := &syncBuf{}
	done := make(chan error, 1)
	go func() { done <- Proxy(ln, permitidos, log) }()
	t.Cleanup(func() {
		ln.Close()
		if err := <-done; err != nil {
			t.Errorf("Proxy returned %v after the listener closed", err)
		}
	})
	return sock, log
}

// eco is a plain TCP server that echoes back what it reads.
func eco(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); io.Copy(c, c) }()
		}
	}()
	return ln.Addr().String()
}

// pedir sends one raw request over c and returns the response status code.
func pedir(t *testing.T, c net.Conn, req string) (int, *bufio.Reader) {
	t.Helper()
	c.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.WriteString(c, req); err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(c)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, br
}

func TestProxyNiegaHostNoPermitido(t *testing.T) {
	sock, log := proxyEnSocket(t, []string{"api.anthropic.com:443"})
	c, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	code, _ := pedir(t, c, "CONNECT example.com:443 HTTP/1.1\r\nHost: example.com:443\r\nProxy-Authorization: Basic c2VjcmV0\r\n\r\n")
	if code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", code)
	}
	if !strings.Contains(log.String(), "DENY example.com:443") {
		t.Fatalf("log %q lacks DENY example.com:443", log.String())
	}
	if strings.Contains(log.String(), "c2VjcmV0") || strings.Contains(log.String(), "Proxy-Authorization") {
		t.Fatalf("the log carries headers: %q", log.String())
	}
}

func TestProxyPermitido(t *testing.T) {
	dst := eco(t)
	sock, log := proxyEnSocket(t, []string{"api.anthropic.com:443", dst})
	c, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	code, br := pedir(t, c, "CONNECT "+dst+" HTTP/1.1\r\nHost: "+dst+"\r\n\r\n")
	if code != http.StatusOK {
		t.Fatalf("status %d, want 200", code)
	}
	if _, err := io.WriteString(c, "ping\n"); err != nil {
		t.Fatal(err)
	}
	line, err := br.ReadString('\n')
	if err != nil || line != "ping\n" {
		t.Fatalf("tunnel echoed %q, %v", line, err)
	}
	if !strings.Contains(log.String(), "ALLOW "+dst) {
		t.Fatalf("log %q lacks ALLOW %s", log.String(), dst)
	}
}

func TestProxyPuertoPorDefecto(t *testing.T) {
	// An entry without a port allows only :443 of that host.
	sock, log := proxyEnSocket(t, []string{"api.anthropic.com"})
	c, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if code, _ := pedir(t, c, "CONNECT api.anthropic.com:80 HTTP/1.1\r\nHost: api.anthropic.com:80\r\n\r\n"); code != http.StatusForbidden {
		t.Fatalf("status %d for :80, want 403", code)
	}
	if !strings.Contains(log.String(), "DENY api.anthropic.com:80") {
		t.Fatalf("log %q", log.String())
	}
}

func TestProxyNoEsHTTPPlano(t *testing.T) {
	dst := eco(t)
	sock, log := proxyEnSocket(t, []string{dst})
	c, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	code, _ := pedir(t, c, "GET http://"+dst+"/ HTTP/1.1\r\nHost: "+dst+"\r\n\r\n")
	if code != http.StatusMethodNotAllowed {
		t.Fatalf("status %d, want 405", code)
	}
	if !strings.Contains(log.String(), "DENY") {
		t.Fatalf("log %q lacks a DENY line", log.String())
	}
}

func TestPuenteReenvia(t *testing.T) {
	dst := eco(t)
	sock, _ := proxyEnSocket(t, []string{dst})
	ln, err := Puente(sock, "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	code, br := pedir(t, c, "CONNECT "+dst+" HTTP/1.1\r\nHost: "+dst+"\r\n\r\n")
	if code != http.StatusOK {
		t.Fatalf("status %d through the bridge, want 200", code)
	}
	io.WriteString(c, "hola\n")
	if line, _ := br.ReadString('\n'); line != "hola\n" {
		t.Fatalf("bridge echoed %q", line)
	}
}

func TestConstruirRedProxy(t *testing.T) {
	h, tmp := hostFalso(t)
	dir := mkdir(t, filepath.Join(tmp, "orq-r1"))
	a, _, err := construir(Spec{Red: RedProxy, Proxy: dir}, h, []string{"true"})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(a, "--share-net") || !slices.Contains(a, "--unshare-all") {
		t.Fatalf("the proxy box has its own netns: %v", a)
	}
	if par(a, "--bind", dir, "/run/orq/red") < 0 {
		t.Fatalf("missing --bind %s /run/orq/red: %v", dir, a)
	}
	if par(a, "--ro-bind", h.Resolv, h.Resolv) >= 0 {
		t.Fatalf("no resolv.conf: DNS is resolved outside by the proxy: %v", a)
	}
	// /run is a tmpfs: the socket dir has to be mounted after it.
	if slices.Index(a, "/run") > par(a, "--bind", dir, "/run/orq/red") {
		t.Fatalf("the socket dir is mounted before the /run tmpfs: %v", a)
	}
}

func TestConstruirRedProxyInvalida(t *testing.T) {
	h, tmp := hostFalso(t)
	dir := mkdir(t, filepath.Join(tmp, "orq-r1"))
	for nombre, s := range map[string]Spec{
		"without a dir":  {Red: RedProxy},
		"relative dir":   {Red: RedProxy, Proxy: "orq-r1"},
		"with host net":  {Red: RedProxy, Proxy: dir, Net: true},
		"dir under /run": {Red: RedProxy, Proxy: "/run/user"},
	} {
		if _, _, err := construir(s, h, []string{"true"}); err == nil {
			t.Errorf("%s: construir accepted %+v", nombre, s)
		}
	}
}

func TestDirSocketCorto(t *testing.T) {
	t.Setenv("TMPDIR", "/tmp/"+strings.Repeat("x", 200))
	dir, err := DirSocket()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if len(dir)+len("/p.sock") >= 108 {
		t.Fatalf("socket path %q is %d bytes, AF_UNIX allows 107", dir+"/p.sock", len(dir)+len("/p.sock"))
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() || fi.Mode().Perm() != 0o700 {
		t.Fatalf("socket dir %s: %v %v", dir, fi, err)
	}
}
