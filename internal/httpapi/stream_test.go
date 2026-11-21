package httpapi

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/BrunaFranca/vida-da-bruna-api/internal/aovivo"
)

// hubFalso guarda o canal da assinatura para o teste "publicar" mudanças à mão.
type hubFalso struct {
	mu sync.Mutex
	c  chan aovivo.Mudanca
}

func (h *hubFalso) Assinar(context.Context, string) (<-chan aovivo.Mudanca, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.c = make(chan aovivo.Mudanca, 1)
	return h.c, func() {}
}

func (h *hubFalso) publicar(m aovivo.Mudanca) {
	for i := 0; i < 100; i++ { // espera a assinatura existir
		h.mu.Lock()
		c := h.c
		h.mu.Unlock()
		if c != nil {
			c <- m
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// proximoEvento lê o próximo "data: ..." do stream SSE.
func proximoEvento(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	for {
		linha, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(linha, "data: ") {
			return strings.TrimSpace(strings.TrimPrefix(linha, "data: "))
		}
	}
}

func TestStreamDoc(t *testing.T) {
	mem := memoria{}
	hub := &hubFalso{}
	srv := httptest.NewServer(New(silencioso, bancoFalso{}, mem, hub))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/config/main/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}
	r := bufio.NewReader(resp.Body)

	if ev := proximoEvento(t, r); ev != "null" {
		t.Fatalf("antes de existir, quero null; veio %s", ev)
	}

	_ = mem.Gravar(ctx, "config", "main", map[string]any{"tema": "rosa"})
	hub.publicar(aovivo.Mudanca{Colecao: "config", ID: "outro"}) // outro doc: não deve gerar evento
	hub.publicar(aovivo.Mudanca{Colecao: "config", ID: "main"})

	if ev := proximoEvento(t, r); !strings.Contains(ev, `"tema":"rosa"`) || !strings.Contains(ev, `"id":"main"`) {
		t.Fatalf("depois de gravar, veio %s", ev)
	}
}
