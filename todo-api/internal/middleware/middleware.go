package middleware

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"time"
)

type ctxKey int

const reqFieldsKey ctxKey = 0

// RequestFields carries per-request values extracted by handlers and logged
// by the logging middleware at the end of the request.
type RequestFields struct {
	UserID string
	TodoID string
}

// Fields returns the RequestFields pointer attached to r's context, or an
// empty value if none is present.
func Fields(r *http.Request) *RequestFields {
	if v, ok := r.Context().Value(reqFieldsKey).(*RequestFields); ok {
		return v
	}
	return &RequestFields{}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wrote {
		r.status = code
		r.wrote = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wrote {
		r.status = http.StatusOK
		r.wrote = true
	}
	return r.ResponseWriter.Write(b)
}

var accessLogger = log.New(os.Stdout, "", 0)

// Logging emits a structured JSON access log for each request. It deliberately
// omits Authorization tokens and request bodies (which may contain titles).
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		fields := &RequestFields{}
		ctx := context.WithValue(r.Context(), reqFieldsKey, fields)
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r.WithContext(ctx))
		latency := time.Since(start)

		entry := map[string]any{
			"ts":         start.UTC().Format(time.RFC3339Nano),
			"method":     r.Method,
			"path":       r.URL.Path,
			"todo_id":    fields.TodoID,
			"user_id":    fields.UserID,
			"status":     rec.status,
			"latency_ms": float64(latency.Microseconds()) / 1000.0,
		}
		if b, err := json.Marshal(entry); err == nil {
			accessLogger.Println(string(b))
		}
	})
}

// Recover converts panics into a 500 JSON response and logs a stack trace.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf(`{"level":"error","msg":"panic","method":%q,"path":%q,"err":%q,"stack":%q}`,
					r.Method, r.URL.Path, toString(rec), string(debug.Stack()))
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"code":"internal_error","message":"internal server error"}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case error:
		return t.Error()
	default:
		return "panic"
	}
}
