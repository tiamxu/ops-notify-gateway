package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequestDeadlinePreservesEarlierCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	want, _ := ctx.Deadline()
	handler := RequestDeadline(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := r.Context().Deadline()
		if !ok || !got.Equal(want) {
			t.Error("overrode parent deadline")
		}
		<-r.Context().Done()
		if r.Context().Err() != context.DeadlineExceeded {
			t.Error("deadline did not cancel request")
		}
	}), time.Second, time.Second)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil).WithContext(ctx))
}

func TestRequestDeadlineAllowsFastResponse(t *testing.T) {
	var requestContext context.Context
	handler := RequestDeadline(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestContext = r.Context()
		w.WriteHeader(http.StatusNoContent)
	}), time.Second, time.Second)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("HTTP %d", rec.Code)
	}
	if requestContext.Err() != context.Canceled {
		t.Fatal("request timer was not released")
	}
}
