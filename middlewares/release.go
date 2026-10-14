package middlewares

import (
	"github.com/labstack/echo/v4"
	"github.com/mavryk-network/mvproxy/config"
)

// ReleaseBalancerTargets tells the balancer once a request is done, so the
// backends it was proxied to stop counting as busy. It must wrap every
// middleware that can reach the proxy (Retry may proxy a request twice).
func ReleaseBalancerTargets(config *config.Config) echo.MiddlewareFunc {
	balancer, ok := config.ProxyConfig.Balancer.(interface{ Release(echo.Context) })
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			if ok {
				defer balancer.Release(c)
			}
			return next(c)
		}
	}
}
