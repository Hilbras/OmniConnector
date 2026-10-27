package handlers

import (
	"context"
	"net/http"
	"regexp"
	"time"

	"github.com/hilbras/omniconnector/packages/kernel-runtime/ids"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/kernel"
	"github.com/hilbras/omniconnector/packages/shared-contracts"
)

// RequestIDHeader carries the correlation ID in requests and responses
const RequestIDHeader = "X-Request-ID"

var clientRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

type identityKey struct{}

func identityFrom(ctx context.Context) *contracts.IdentityContext {
	id, _ := ctx.Value(identityKey{}).(*contracts.IdentityContext)
	return id
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// withObservability assigns the request ID, recovers panics, and logs each request
func (h *Handler) withObservability(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get(RequestIDHeader)
		if !clientRequestID.MatchString(rid) {
			rid = ids.WithPrefix("req")
		}
		w.Header().Set(RequestIDHeader, rid)
		r = r.WithContext(kernel.WithRequestID(r.Context(), rid))

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		defer func() {
			if p := recover(); p != nil {
				h.log.Error("handler panicked", "request_id", rid, "panic", p)
				writeError(rec, r, http.StatusInternalServerError, contracts.NewError(contracts.ErrorCategoryInternal, "internal_error", "internal server error"))
			}
			h.log.Info("http request", "request_id", rid, "method", r.Method, "path", r.URL.Path,
				"status", rec.status, "duration_ms", time.Since(start).Milliseconds())
		}()
		next.ServeHTTP(rec, r)
	})
}

// authenticated rejects requests without a valid caller identity
func (h *Handler) authenticated(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := h.authn.Authenticate(r)
		if err != nil || caller == nil {
			w.Header().Set("WWW-Authenticate", `Bearer realm="omniconnector"`)
			writeError(w, r, http.StatusUnauthorized, contracts.NewError(contracts.ErrorCategoryAuthFailed, "unauthenticated", "a valid bearer token is required"))
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), identityKey{}, caller)))
	}
}
