package httprecover

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

func RecoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				stackTrace := debug.Stack()
				slog.Error("handler panic",
					slog.Any("err", err),
					slog.String("stacktrace", string(stackTrace)),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
				)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
