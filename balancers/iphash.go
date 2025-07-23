package balancers

import (
	"math/rand"
	"sync"
	"time"

	echocache "github.com/fraidev/go-echo-cache"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"log"
)

type ipHashBalancer struct {
	targets     []*middleware.ProxyTarget
	retryTarget *middleware.ProxyTarget
	mutex       sync.Mutex
	random      *rand.Rand
	store       echocache.Cache
	TTL         int
	activeReqs sync.Map // map[string]int, backend address -> active requests
}

func NewIPHashBalancer(targets []*middleware.ProxyTarget, retryTarget *middleware.ProxyTarget, ttl int, store echocache.Cache) middleware.ProxyBalancer {
	b := ipHashBalancer{}
	b.targets = targets
	b.retryTarget = retryTarget
	b.random = rand.New(rand.NewSource(int64(time.Now().Nanosecond())))
	b.store = store
	b.TTL = ttl
	return &b
}

func (b *ipHashBalancer) AddTarget(target *middleware.ProxyTarget) bool {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	for _, t := range b.targets {
		if t.Name == target.Name {
			return false
		}
	}
	b.targets = append(b.targets, target)
	return true
}

func (b *ipHashBalancer) RemoveTarget(name string) bool {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	for i, t := range b.targets {
		if t.Name == name {
			b.targets = append(b.targets[:i], b.targets[i+1:]...)
			return true
		}
	}
	return false
}

func (b *ipHashBalancer) SetTargets(targets []*middleware.ProxyTarget) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.targets = targets
}

func (b *ipHashBalancer) Next(c echo.Context) *middleware.ProxyTarget {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if len(b.targets) == 0 {
		return nil
	} else if len(b.targets) == 1 && b.retryTarget == nil {
		log.Printf("[mvproxy] %s -> %s %s", c.RealIP(), b.targets[0].URL.String(), c.Request().URL.Path)
		b.incActive(b.targets[0].URL.Host)
		return b.targets[0]
	}

	if c.Get("retry") != nil {
		log.Printf("[mvproxy] %s -> %s (retry) %s", c.RealIP(), b.retryTarget.URL.String(), c.Request().URL.Path)
		b.incActive(b.retryTarget.URL.Host)
		return b.retryTarget
	}

	ctx := c.Request().Context()
	ip := []byte(c.RealIP())
	got, err := b.store.Get(ctx, ip)
	var target *middleware.ProxyTarget
	if err == nil && len(got) > 0 {
		backendAddr := string(got)
		for _, t := range b.targets {
			if t.URL.Host == backendAddr {
				target = t
				break
			}
		}
	}
	if target == nil {
		// Least-connections: pick backend with fewest active requests
		minReqs := int(^uint(0) >> 1) // max int
		for _, t := range b.targets {
			v, _ := b.activeReqs.LoadOrStore(t.URL.Host, 0)
			if reqs, ok := v.(int); ok && reqs < minReqs {
				minReqs = reqs
				target = t
			}
		}
		if target == nil {
			i := b.random.Intn(len(b.targets))
			target = b.targets[i]
		}
		b.store.Set(ctx, ip, []byte(target.URL.Host), b.TTL)
	}

	b.incActive(target.URL.Host)
	log.Printf("[mvproxy] %s -> %s %s", c.RealIP(), target.URL.String(), c.Request().URL.Path)
	return target
}

// Call this when a request is done (in a middleware after proxying)
func (b *ipHashBalancer) Done(host string) {
	v, _ := b.activeReqs.LoadOrStore(host, 0)
	if reqs, ok := v.(int); ok && reqs > 0 {
		b.activeReqs.Store(host, reqs-1)
	}
}

func (b *ipHashBalancer) incActive(host string) {
	v, _ := b.activeReqs.LoadOrStore(host, 0)
	b.activeReqs.Store(host, v.(int)+1)
}
