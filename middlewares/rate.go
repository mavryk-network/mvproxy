package middlewares

import (
	"log"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/mavryk-network/mvproxy/config"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
	"github.com/ulule/limiter/v3/drivers/store/redis"
)

func RateLimit(config *config.Config) echo.MiddlewareFunc {
	var store limiter.Store
	if config.ConfigFile.Redis.Enabled {
		var err error
		store, err = redis.NewStore(config.Redis)
		if err != nil {
			// Exiting here crash-looped the proxy whenever redis or cluster DNS
			// was briefly unreachable at startup; rate limit per process instead.
			log.Printf("[mvproxy] rate limiter: redis store unavailable (%v), using in-memory store", err)
			store = memory.NewStore()
		}
	} else {
		store = memory.NewStore()
	}

	ipRateLimiter := limiter.New(store, *config.Rate)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			if !config.ConfigFile.RateLimit.Enabled {
				return next(c)
			}
			ip := c.RealIP()
			limiterCtx, err := ipRateLimiter.Get(c.Request().Context(), ip)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, echo.Map{
					"success": false,
					"message": err,
				})
			}

			h := c.Response().Header()
			h.Set("X-RateLimit-Limit", strconv.FormatInt(limiterCtx.Limit, 10))
			h.Set("X-RateLimit-Remaining", strconv.FormatInt(limiterCtx.Remaining, 10))
			h.Set("X-RateLimit-Reset", strconv.FormatInt(limiterCtx.Reset, 10))

			if limiterCtx.Reached {
				return c.JSON(http.StatusTooManyRequests, echo.Map{
					"success": false,
					"message": "Too Many Requests on " + c.Request().URL.String(),
				})
			}

			return next(c)
		}
	}
}
