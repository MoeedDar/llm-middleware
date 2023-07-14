package main

import (
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

type responseWriter struct {
	http.ResponseWriter
	StatusCode int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.StatusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func logWriteErr(w http.ResponseWriter, id string, err error, msg string, status int) {
	log.Error().
		Str("request_id", id).
		Int("status", status).
		Err(err).
		Msg(msg)
	http.Error(w, msg, status)
}

// Not working with SSE streaming?
func loggerMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		logger := log.With().Str("request_id", r.Header.Get("X-Request-ID")).Logger()
		logger.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Msg("received")

		rw := &responseWriter{ResponseWriter: w}
		next(rw, r)

		logger.Info().
			Int("status", rw.StatusCode).
			Dur("response_time", time.Since(startTime)).
			Msg("completed")
	}
}
