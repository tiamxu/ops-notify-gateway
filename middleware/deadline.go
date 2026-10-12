package middleware

import (
	"context"
	"net/http"
	"time"
)

func RequestDeadline(next http.Handler, budget, readTimeout time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), budget)
		defer cancel()
		deadline, _ := ctx.Deadline()
		if readTimeout > 0 {
			readDeadline := time.Now().Add(readTimeout)
			if readDeadline.Before(deadline) {
				deadline = readDeadline
			}
		}
		controller := http.NewResponseController(w)
		_ = controller.SetReadDeadline(deadline)
		// Keep the deadline while net/http drains an unfinished body after the handler.
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
