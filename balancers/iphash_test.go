package balancers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/coocood/freecache"
	echocache "github.com/fraidev/go-echo-cache"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func newBalancer(t *testing.T, hosts ...string) *ipHashBalancer {
	t.Helper()
	var targets []*middleware.ProxyTarget
	for _, h := range hosts {
		u, err := url.Parse("http://" + h)
		if err != nil {
			t.Fatal(err)
		}
		targets = append(targets, &middleware.ProxyTarget{Name: h, URL: u})
	}
	store := echocache.NewMemoryCache(freecache.NewCache(1024 * 1024))
	return NewIPHashBalancer(targets, nil, 600, &store).(*ipHashBalancer)
}

func newContext(clientIP string) echo.Context {
	req := httptest.NewRequest(http.MethodGet, "/chains/main/blocks/head/header", nil)
	req.Header.Set(echo.HeaderXRealIP, clientIP)
	return echo.New().NewContext(req, httptest.NewRecorder())
}

// 2026-10-14: requests to two hung RPC nodes never completed, so those nodes
// had received the fewest requests and every re-pinned client was sent to them.
func TestNewClientsAvoidBackendWithHungRequest(t *testing.T) {
	b := newBalancer(t, "10.0.0.1:8732", "10.0.0.2:8732")

	hung := b.Next(newContext("192.0.2.1")) // never released

	for i := 0; i < 50; i++ {
		c := newContext("192.0.2.2")
		if got := b.Next(c); got.URL.Host == hung.URL.Host {
			t.Fatalf("request %d of a new client went to the backend with an in-flight request", i)
		}
		b.Release(c)
	}

	if got := b.Next(newContext("192.0.2.3")); got.URL.Host == hung.URL.Host {
		t.Fatalf("new client pinned to %s, which still has a hung request, over an idle backend", got.URL.Host)
	}
}

func TestStickyClientKeepsBackend(t *testing.T) {
	b := newBalancer(t, "10.0.0.1:8732", "10.0.0.2:8732", "10.0.0.3:8732")

	c := newContext("192.0.2.1")
	first := b.Next(c)
	b.Release(c)
	for i := 0; i < 10; i++ {
		c := newContext("192.0.2.1")
		if got := b.Next(c); got.URL.Host != first.URL.Host {
			t.Fatalf("client moved from %s to %s", first.URL.Host, got.URL.Host)
		}
		b.Release(c)
	}
}

func TestRetryAvoidsFailedBackendAndRepins(t *testing.T) {
	b := newBalancer(t, "10.0.0.1:8732", "10.0.0.2:8732")

	c := newContext("192.0.2.1")
	first := b.Next(c)
	// What echo's proxy leaves in the context before retrying a failed attempt.
	c.Set("target", first)
	c.Set("_error", echo.NewHTTPError(http.StatusBadGateway))
	retry := b.Next(c)
	if retry.URL.Host == first.URL.Host {
		t.Fatalf("retry sent back to the failed backend %s", first.URL.Host)
	}
	b.Release(c)

	c = newContext("192.0.2.1")
	if got := b.Next(c); got.URL.Host != retry.URL.Host {
		t.Fatalf("client not re-pinned after failover: got %s, want %s", got.URL.Host, retry.URL.Host)
	}
	b.Release(c)

	for host, n := range b.activeReqs {
		if n != 0 {
			t.Fatalf("%d requests still counted in flight on %s after release", n, host)
		}
	}
}

func TestConcurrentNextAndRelease(t *testing.T) {
	b := newBalancer(t, "10.0.0.1:8732", "10.0.0.2:8732", "10.0.0.3:8732")

	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c := newContext(fmt.Sprintf("192.0.2.%d", i))
				b.Next(c)
				b.Release(c)
			}
		}(i)
	}
	wg.Wait()

	for host, n := range b.activeReqs {
		if n != 0 {
			t.Fatalf("%d requests still counted in flight on %s", n, host)
		}
	}
}
