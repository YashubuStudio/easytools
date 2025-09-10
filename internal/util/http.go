package util

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func WriteJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func LogMiddleware(next http.Handler, w io.Writer) http.Handler {
	if w == nil {
		return next
	}
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		t := time.Now()
		next.ServeHTTP(rw, r)
		Logf(w, "[http] %s %s %s\n", r.Method, r.URL.Path, time.Since(t))
	})
}

func Logf(w io.Writer, format string, args ...any) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprintf(w, format, args...)
}
