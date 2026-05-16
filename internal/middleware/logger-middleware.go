package middleware

import (
	"net/http"
	"time"

	"github.com/ustasjs/gopher-markt/internal/logger"
	"go.uber.org/zap"
)

type (
	responseData struct {
		status int
		size   int
	}
	loggingResponseWriter struct {
		http.ResponseWriter
		responseData *responseData
	}
)

func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	r.responseData.size += size
	return size, err
}

func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode
}

func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		uri := r.RequestURI
		method := r.Method

		responseData := &responseData{
			status: 0,
			size:   0,
		}
		lw := loggingResponseWriter{
			ResponseWriter: w,
			responseData:   responseData,
		}

		next.ServeHTTP(&lw, r)

		duration := time.Since(start)

		logger.Log.Info("request", zap.String("uri", uri), zap.String("method", method), zap.Duration("duration", duration))
		logger.Log.Info("responce", zap.Int("status", responseData.status), zap.Int("size", responseData.size))
	})
}
