package main

import (
	"context"
	"net/http"
	"net/http/pprof"
	"net"
	"strings"
	"os"
	"os/signal"
	"runtime/debug"
	"time"
	"log"

	"github.com/fraidev/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/mavryk-network/mvproxy/config"
	"github.com/mavryk-network/mvproxy/middlewares"
	"github.com/ziflex/lecho/v3"
)

func main() {
	config := config.NewConfig()

	debug.SetGCPercent(config.ConfigFile.GC.Percent)

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Middlewares
	e.Logger = lecho.From(config.Logger)
	e.Use(middleware.Recover())
	e.Use(middleware.RequestLoggerWithConfig(*config.RequestLoggerConfig))
	e.Use(middlewares.CORS(config))
	e.Use(middlewares.RateLimit(config))
	e.Use(middlewares.DenyIPs(config))
	e.Use(middlewares.AllowRoutes(config))
	e.Use(middlewares.DenyRoutes(config))
	e.Use(middlewares.Cache(config))
	e.Use(middlewares.Gzip(config))
	e.Use(middlewares.Retry(config))
	e.Use(middleware.ProxyWithConfig(*config.ProxyConfig))

	// Start metrics server
	startMetricsServer(config)

	// Start dynamic DNS resolver for mavryk_host
	go func() {
		interval := 30 * time.Second
		for {
			var newTargets []*middleware.ProxyTarget
			for _, host := range config.ConfigFile.MavrykHost {
				if net.ParseIP(host) == nil && !strings.Contains(host, ":") {
					// Host is a DNS name, resolve all A records
					ips, err := net.LookupHost(host)
					if err == nil {
						for _, ip := range ips {
							newTargets = append(newTargets, config.HostToTarget(ip+":8732")) // default port, adjust as needed
						}
					}
				} else {
					newTargets = append(newTargets, config.HostToTarget(host))
				}
			}
			if len(newTargets) > 0 {
				var urls []string
				for _, t := range newTargets {
					urls = append(urls, t.URL.String())
				}
				log.Printf("[mvproxy] Updating balancer targets: %v", urls)
				if balancer, ok := config.ProxyConfig.Balancer.(interface{ SetTargets([]*middleware.ProxyTarget) }); ok {
					balancer.SetTargets(newTargets)
				}
			}
			time.Sleep(interval)
		}
	}()

	// Start proxy
	startProxyWithGracefulShutdown(e, config)
}

func startMetricsServer(config *config.Config) {
	if config.ConfigFile.Metrics.Enabled {
		go func() {
			metrics := echo.New()

			if config.ConfigFile.Metrics.Pprof {
				pp := http.NewServeMux()
				pp.HandleFunc("/debug/pprof/", pprof.Index)
				pp.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
				pp.HandleFunc("/debug/pprof/profile", pprof.Profile)
				pp.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
				pp.HandleFunc("/debug/pprof/trace", pprof.Trace)
				metrics.GET("/debug/pprof/*", echo.WrapHandler(pp))
			}
			metrics.GET("/metrics", echoprometheus.NewHandler())
			metrics.HideBanner = true
			metrics.HidePort = true
			if err := metrics.Start(config.ConfigFile.Metrics.Host); err != nil && err != http.ErrServerClosed {
				metrics.Logger.Fatal(err)
			}
		}()
	}

}

func startProxyWithGracefulShutdown(e *echo.Echo, config *config.Config) {
	go func() {
		if err := e.Start(config.ConfigFile.Host); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("Shutting down the server")
			e.Logger.Fatal(err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds.
	// Use a buffered channel to avoid missing signals as recommended for signal.Notify
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}
}
