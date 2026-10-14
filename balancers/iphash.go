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

// pickedHostsKey is the echo context key under which Next records every
// backend it hands out for a request, so that Release can return them all.
const pickedHostsKey = "mvproxy.balancer.picked_hosts"

type ipHashBalancer struct {
	targets     []*middleware.ProxyTarget
	retryTarget *middleware.ProxyTarget
	mutex       sync.Mutex
	random      *rand.Rand
	store       echocache.Cache
	TTL         int
	activeReqs  map[string]int // backend address -> in-flight requests, guarded by mutex
}

func NewIPHashBalancer(targets []*middleware.ProxyTarget, retryTarget *middleware.ProxyTarget, ttl int, store echocache.Cache) middleware.ProxyBalancer {
	b := ipHashBalancer{}
	b.targets = targets
	b.retryTarget = retryTarget
	b.random = rand.New(rand.NewSource(int64(time.Now().Nanosecond())))
	b.store = store
	b.TTL = ttl
	b.activeReqs = make(map[string]int)
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
		b.incActive(c, b.targets[0].URL.Host)
		return b.targets[0]
	}

	if c.Get("retry") != nil && b.retryTarget != nil {
		log.Printf("[mvproxy] %s -> %s (retry) %s", c.RealIP(), b.retryTarget.URL.String(), c.Request().URL.Path)
		b.incActive(c, b.retryTarget.URL.Host)
		return b.retryTarget
	}

	// When echo's proxy retries a failed attempt (e.g. a 502 from a dead
	// backend), the context still holds the previous target and its error.
	// Don't send the retry, nor this client's next requests, to that backend.
	failedHost := ""
	if _, retrying := c.Get("_error").(error); retrying {
		if prev, ok := c.Get("target").(*middleware.ProxyTarget); ok && prev != nil {
			failedHost = prev.URL.Host
		}
	}

	ctx := c.Request().Context()
	ip := []byte(c.RealIP())
	got, err := b.store.Get(ctx, ip)
	var target *middleware.ProxyTarget
	if err == nil && len(got) > 0 && string(got) != failedHost {
		backendAddr := string(got)
		for _, t := range b.targets {
			if t.URL.Host == backendAddr {
				target = t
				break
			}
		}
	}
	if target == nil {
		target = b.leastActive(failedHost)
		b.store.Set(ctx, ip, []byte(target.URL.Host), b.TTL)
	}

	b.incActive(c, target.URL.Host)
	log.Printf("[mvproxy] %s -> %s %s", c.RealIP(), target.URL.String(), c.Request().URL.Path)
	return target
}

// Least-connections: the backend with the fewest in-flight requests, ties
// broken randomly. Skips `exclude` unless it is the only backend.
func (b *ipHashBalancer) leastActive(exclude string) *middleware.ProxyTarget {
	var best []*middleware.ProxyTarget
	minReqs := int(^uint(0) >> 1) // max int
	for _, t := range b.targets {
		if t.URL.Host == exclude && len(b.targets) > 1 {
			continue
		}
		reqs := b.activeReqs[t.URL.Host]
		if reqs < minReqs {
			minReqs = reqs
			best = best[:0]
		}
		if reqs == minReqs {
			best = append(best, t)
		}
	}
	return best[b.random.Intn(len(best))]
}

// Must be called with b.mutex held. Records the host in the request context
// so that Release can decrement it once the request is done.
func (b *ipHashBalancer) incActive(c echo.Context, host string) {
	b.activeReqs[host]++
	picked, _ := c.Get(pickedHostsKey).([]string)
	c.Set(pickedHostsKey, append(picked, host))
}

// Done marks one request to host as finished.
func (b *ipHashBalancer) Done(host string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.activeReqs[host] > 0 {
		b.activeReqs[host]--
	}
}

// Release marks every backend Next picked for this request (proxy retries
// included) as finished. It must run once the request is done -- see
// middlewares.ReleaseBalancerTargets. Without it the counters only grow, and
// least-connections sends new clients to the backends that served the fewest
// requests so far: the slow or hung ones.
func (b *ipHashBalancer) Release(c echo.Context) {
	picked, _ := c.Get(pickedHostsKey).([]string)
	c.Set(pickedHostsKey, nil)
	for _, host := range picked {
		b.Done(host)
	}
}
