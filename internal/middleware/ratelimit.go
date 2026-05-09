package middleware

import (
	"strconv"
	"sync"
	"time"

	"github.com/CodeEnthusiast09/proctura-backend/internal/response"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// PerIP returns a Gin middleware that limits each client IP to a token bucket
// of `rps` refills per second with a `burst` ceiling. It tracks a separate
// rate.Limiter per IP in an unbounded sync.Map — fine for a single API process
// at school-scale traffic. If you ever run more than one API instance, swap
// this for a Redis-backed limiter so the buckets are shared.
//
// Returns 429 with a Retry-After header (in seconds) when the bucket is empty.
func PerIP(rps rate.Limit, burst int) gin.HandlerFunc {
	var clients sync.Map // key: client IP, value: *rate.Limiter

	getLimiter := func(ip string) *rate.Limiter {
		if v, ok := clients.Load(ip); ok {
			return v.(*rate.Limiter)
		}
		lim := rate.NewLimiter(rps, burst)
		actual, _ := clients.LoadOrStore(ip, lim)
		return actual.(*rate.Limiter)
	}

	return func(c *gin.Context) {
		lim := getLimiter(c.ClientIP())
		reservation := lim.Reserve()
		if !reservation.OK() {
			// Bucket is impossibly small — should never happen with sane config.
			c.Header("Retry-After", "60")
			response.TooManyRequests(c, "rate limit exceeded")
			c.Abort()
			return
		}
		delay := reservation.Delay()
		if delay > 0 {
			// Token not yet available — surface as 429 instead of waiting in
			// the request goroutine. The client can retry after Retry-After.
			reservation.Cancel()
			seconds := max(int(delay.Round(time.Second)/time.Second), 1)
			c.Header("Retry-After", strconv.Itoa(seconds))
			response.TooManyRequests(c, "too many requests — please try again shortly")
			c.Abort()
			return
		}
		c.Next()
	}
}
