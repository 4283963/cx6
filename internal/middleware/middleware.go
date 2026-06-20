package middleware

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method
		clientIP := c.ClientIP()

		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		rw := &responseWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBufferString(""),
		}
		c.Writer = rw

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		log.Printf("[HTTP] %s | %3d | %13v | %15s | %s %s | body=%s | resp=%s",
			time.Now().Format("2006-01-02 15:04:05"),
			statusCode,
			latency,
			clientIP,
			method,
			path,
			truncateString(string(bodyBytes), 200),
			truncateString(rw.body.String(), 200),
		)
	}
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func APISignAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != "POST" {
			c.Next()
			return
		}

		timestamp := c.GetHeader("X-Timestamp")
		sign := c.GetHeader("X-Sign")

		if timestamp == "" || sign == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code": 40101,
				"msg":  "missing auth headers",
			})
			c.Abort()
			return
		}

		now := time.Now().Unix()
		reqTime := int64(0)
		if err := parseTime(timestamp, &reqTime); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code": 40102,
				"msg":  "invalid timestamp format",
			})
			c.Abort()
			return
		}

		if abs(now-reqTime) > 300 {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code": 40103,
				"msg":  "timestamp expired",
			})
			c.Abort()
			return
		}

		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code": 40001,
				"msg":  "invalid request body",
			})
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		expectedSign := generateAPISign(c.Request.Method, c.Request.URL.Path, timestamp, bodyBytes, secret)
		if sign != expectedSign {
			log.Printf("[AUTH] sign mismatch: path=%s, expected=%s, actual=%s",
				c.Request.URL.Path, expectedSign, sign)
			c.JSON(http.StatusUnauthorized, gin.H{
				"code": 40104,
				"msg":  "invalid signature",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func RateLimitByIP(limitPerMinute int) gin.HandlerFunc {
	var mu sync.Mutex
	bucket := make(map[string]*rateBucket)
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		mu.Lock()
		b, ok := bucket[ip]
		if !ok || now.After(b.resetTime) {
			bucket[ip] = &rateBucket{
				count:     1,
				resetTime: now.Add(time.Minute),
			}
			mu.Unlock()
			c.Next()
			return
		}

		b.count++
		current := b.count
		mu.Unlock()

		if current > limitPerMinute {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code": 42901,
				"msg":  "too many requests",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[PANIC] recovered from panic: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"code": 50000,
					"msg":  "internal server error",
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

type rateBucket struct {
	count     int
	resetTime time.Time
}

func parseTime(s string, out *int64) error {
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*out = val
	return nil
}

func generateAPISign(method, path, timestamp string, body []byte, secret string) string {
	raw := method + path + timestamp + string(body) + secret
	h := md5.Sum([]byte(raw))
	return hex.EncodeToString(h[:])
}
