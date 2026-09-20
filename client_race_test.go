package tusgo

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

func TestUpdateCapabilitiesConcurrent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Tus-Resumable", "1.0.0")
		w.Header().Set("Tus-Extension", "creation,termination")
		w.Header().Set("Tus-Version", "1.0.0")
		w.Header().Set("Tus-Max-Size", "1073741824")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c := NewClient(srv.Client(), u)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 40; j++ {
				if _, err := c.UpdateCapabilities(); err != nil {
					t.Errorf("UpdateCapabilities: %v", err)
					return
				}
				if err := c.ensureExtension("creation"); err != nil {
					t.Errorf("ensureExtension: %v", err)
					return
				}
				cc := c.WithContext(c.ctx)
				if err := cc.ensureExtension("creation"); err != nil {
					t.Errorf("copy ensureExtension: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	caps := c.capabilities()
	if caps == nil || caps.MaxSize != 1073741824 {
		t.Fatalf("unexpected capabilities: %+v", caps)
	}
}
