package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/response"
)

// RateLimiter giới hạn số yêu cầu theo địa chỉ IP bằng thuật toán gàu token.
//
// Bộ nhớ trong là đủ cho phiên bản một node; khi lên đa node, phần trạng thái này
// sẽ chuyển sang kho dùng chung.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket

	// rate là số token nạp lại mỗi giây, burst là sức chứa tối đa của gàu.
	rate  float64
	burst float64
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// NewRateLimiter tạo bộ giới hạn cho phép trung bình ratePerMinute yêu cầu mỗi
// phút, với khả năng dồn cục tối đa burst yêu cầu.
func NewRateLimiter(ratePerMinute, burst int) *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*bucket),
		rate:    float64(ratePerMinute) / 60,
		burst:   float64(burst),
	}
	go rl.cleanupLoop()
	return rl
}

// Middleware trả về lớp trung gian áp dụng giới hạn tần suất.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.allow(c.ClientIP()) {
			response.Fail(c, apperr.TooManyRequests)
			c.Abort()
			return
		}
		c.Next()
	}
}

func (rl *RateLimiter) allow(key string) bool {
	now := time.Now()

	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[key]
	if !ok {
		rl.buckets[key] = &bucket{tokens: rl.burst - 1, lastSeen: now}
		return true
	}

	// Nạp lại token theo thời gian đã trôi qua kể từ lần gọi trước.
	b.tokens += now.Sub(b.lastSeen).Seconds() * rl.rate
	if b.tokens > rl.burst {
		b.tokens = rl.burst
	}
	b.lastSeen = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// cleanupLoop định kỳ xóa các gàu không còn dùng, tránh việc bảng băm phình lên
// vô hạn khi bị quét từ hàng loạt địa chỉ IP khác nhau.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().Add(-30 * time.Minute)
		rl.mu.Lock()
		for key, b := range rl.buckets {
			if b.lastSeen.Before(cutoff) {
				delete(rl.buckets, key)
			}
		}
		rl.mu.Unlock()
	}
}
