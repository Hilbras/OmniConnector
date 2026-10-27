package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	testintegration "github.com/hilbras/omniconnector/integrations/test-integration"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/config"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/kernel"
	"github.com/hilbras/omniconnector/packages/kernel-runtime/logger"
	"github.com/hilbras/omniconnector/services/kernel-api/internal/authn"
)

const devToken = "test-token-0123456789abcdef0123456789"

type testServer struct {
	*httptest.Server
	kernel *kernel.Kernel
}

func newServer(t *testing.T, permissions ...string) *testServer {
	t.Helper()
	cfg := &config.Config{Execution: config.ExecutionConfig{MaxConcurrent: 4, Timeout: 30, MaxOutputBytes: 1 << 20}}
	log := logger.NewWithWriter(io.Discard, "error")
	k, err := kernel.New(cfg, log)
	if err != nil {
		t.Fatal(err)
	}
	if err := k.RegisterPackage(testintegration.NewIntegration()); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(k, log, authn.NewDevToken(devToken, "dev-user", permissions)).Routes())
	t.Cleanup(func() {
		srv.Close()
		_ = k.Shutdown(context.Background())
	})
	return &testServer{Server: srv, kernel: k}
}

func (s *testServer) do(t *testing.T, method, path, body string, headers ...string) (*http.Response, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(method, s.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+devToken)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("response is not JSON: %s", raw)
		}
	}
	return resp, out
}

func errorCode(body map[string]any) string {
	e, _ := body["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

var allPerms = []string{"operation.execute", "operation.cancel", "test.read", "test.write", "audit.read"}

func TestHealthIsPublic(t *testing.T) {
	s := newServer(t)
	resp, err := http.Get(s.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if resp.Header.Get(RequestIDHeader) == "" {
		t.Error("every response must carry a request ID")
	}
}

func TestAuthenticationRequired(t *testing.T) {
	s := newServer(t, allPerms...)
	for _, tc := range []struct{ name, header string }{
		{"missing", ""},
		{"wrong token", "Bearer nope"},
		{"wrong scheme", "Basic " + devToken},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", s.URL+"/v1/integrations", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", resp.StatusCode)
			}
		})
	}
}

func TestUnconfiguredTokenRejectsEverything(t *testing.T) {
	a := authn.NewDevToken("", "dev-user", allPerms)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer ")
	if _, err := a.Authenticate(req); err == nil {
		t.Fatal("an empty dev token must not authenticate anything")
	}
}

func TestIntegrationDiscovery(t *testing.T) {
	s := newServer(t, allPerms...)
	resp, body := s.do(t, "GET", "/v1/integrations", "")
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	list, _ := body["integrations"].([]any)
	if len(list) != 1 {
		t.Fatalf("expected 1 integration, got %v", body)
	}

	resp, body = s.do(t, "GET", "/v1/integrations/test-integration/capabilities", "")
	if resp.StatusCode != 200 || len(body["capabilities"].([]any)) != 2 {
		t.Fatalf("unexpected capabilities response %d %v", resp.StatusCode, body)
	}

	resp, body = s.do(t, "GET", "/v1/integrations/nope", "")
	if resp.StatusCode != 404 || errorCode(body) != "integration_not_found" {
		t.Fatalf("expected 404 integration_not_found, got %d %v", resp.StatusCode, body)
	}
}

// TestPhase1ExitCriteria walks the Phase 1 exit criterion through the API: a
// test integration registers, exposes capabilities, receives a valid
// operation, returns a normalized result, and reports failures correctly.
func TestPhase1ExitCriteria(t *testing.T) {
	s := newServer(t, allPerms...)

	resp, body := s.do(t, "POST", "/v1/operations",
		`{"integration":"test-integration","operationId":"echo","input":{"message":"hello"}}`, RequestIDHeader, "client-req-1")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d %v", resp.StatusCode, body)
	}
	if body["state"] != "succeeded" || body["result"].(map[string]any)["message"] != "hello" {
		t.Fatalf("unexpected operation: %v", body)
	}
	if body["requestId"] != "client-req-1" || resp.Header.Get(RequestIDHeader) != "client-req-1" {
		t.Errorf("client request ID was not propagated: %v", body["requestId"])
	}
	id := body["id"].(string)
	if resp.Header.Get("Location") != "/v1/operations/"+id {
		t.Errorf("unexpected Location %q", resp.Header.Get("Location"))
	}

	resp, body = s.do(t, "GET", "/v1/operations/"+id, "")
	if resp.StatusCode != 200 || body["state"] != "succeeded" {
		t.Fatalf("stored operation not retrievable: %d %v", resp.StatusCode, body)
	}

	resp, body = s.do(t, "POST", "/v1/operations", `{"integration":"test-integration","operationId":"fail","input":{"category":"rate_limited"}}`)
	if resp.StatusCode != http.StatusCreated || body["state"] != "failed" {
		t.Fatalf("expected recorded failure, got %d %v", resp.StatusCode, body)
	}
	if e := body["error"].(map[string]any); e["category"] != "rate_limited" || e["retryable"] != true {
		t.Fatalf("failure was not normalized: %v", e)
	}

	resp, body = s.do(t, "GET", "/v1/operations", "")
	if resp.StatusCode != 200 || len(body["operations"].([]any)) != 2 {
		t.Fatalf("expected 2 operations in history, got %v", body)
	}
}

func TestOperationRejections(t *testing.T) {
	s := newServer(t, "operation.execute", "test.read")
	cases := []struct {
		name, body string
		status     int
		code       string
	}{
		{"malformed json", `{`, 400, "invalid_json"},
		{"unknown field", `{"integration":"test-integration","operationId":"echo","bogus":1}`, 400, "invalid_json"},
		{"trailing data", `{"integration":"test-integration","operationId":"echo","input":{"message":"x"}} {}`, 400, "invalid_json"},
		{"missing fields", `{}`, 400, "invalid_request"},
		{"unknown integration", `{"integration":"nope","operationId":"echo"}`, 404, "integration_not_found"},
		{"unknown operation", `{"integration":"test-integration","operationId":"nope"}`, 404, "operation_not_found"},
		{"schema violation", `{"integration":"test-integration","operationId":"echo","input":{"message":5}}`, 400, "invalid_input"},
		{"missing permission", `{"integration":"test-integration","operationId":"write","input":{}}`, 403, "permission_denied"},
		{"negative timeout", `{"integration":"test-integration","operationId":"echo","timeoutSeconds":-1}`, 400, "invalid_request"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := s.do(t, "POST", "/v1/operations", tc.body)
			if resp.StatusCode != tc.status || errorCode(body) != tc.code {
				t.Fatalf("expected %d %s, got %d %v", tc.status, tc.code, resp.StatusCode, body)
			}
			if body["error"].(map[string]any)["requestId"] == "" {
				t.Error("error envelope must include the request ID")
			}
		})
	}

	t.Run("body too large", func(t *testing.T) {
		big := `{"integration":"test-integration","operationId":"echo","input":{"message":"` + strings.Repeat("a", 2<<20) + `"}}`
		resp, body := s.do(t, "POST", "/v1/operations", big)
		if resp.StatusCode != 400 || errorCode(body) != "body_too_large" {
			t.Fatalf("expected 400 body_too_large, got %d %v", resp.StatusCode, errorCode(body))
		}
	})
}

func TestIdempotencyKeyHeader(t *testing.T) {
	s := newServer(t, allPerms...)
	body := `{"integration":"test-integration","operationId":"write","input":{}}`
	_, first := s.do(t, "POST", "/v1/operations", body, "Idempotency-Key", "abc")
	_, second := s.do(t, "POST", "/v1/operations", body, "Idempotency-Key", "abc")
	if first["id"] != second["id"] {
		t.Fatalf("same Idempotency-Key must return the same operation: %v vs %v", first["id"], second["id"])
	}
}

func TestCancelThroughAPI(t *testing.T) {
	s := newServer(t, allPerms...)

	done := make(chan map[string]any, 1)
	go func() {
		_, body := s.do(t, "POST", "/v1/operations", `{"integration":"test-integration","operationId":"write","input":{"milliseconds":10000}}`)
		done <- body
	}()

	var id string
	deadline := time.Now().Add(2 * time.Second)
	for id == "" && time.Now().Before(deadline) {
		_, list := s.do(t, "GET", "/v1/operations", "")
		for _, o := range list["operations"].([]any) {
			if op := o.(map[string]any); op["state"] == "running" {
				id = op["id"].(string)
			}
		}
	}
	if id == "" {
		t.Fatal("operation never reached running")
	}

	resp, body := s.do(t, "POST", "/v1/operations/"+id+"/cancel", "")
	if resp.StatusCode != http.StatusAccepted || body["state"] != "cancel_requested" {
		t.Fatalf("expected 202 cancel_requested, got %d %v", resp.StatusCode, body)
	}
	final := <-done
	if final["state"] != "unknown_outcome" {
		t.Fatalf("cancelled write must end in unknown_outcome, got %v", final["state"])
	}

	resp, body = s.do(t, "POST", "/v1/operations/"+id+"/cancel", "")
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("cancelling a finished operation should be 409, got %d %v", resp.StatusCode, body)
	}
}

func TestAuditEvents(t *testing.T) {
	s := newServer(t, allPerms...)
	s.do(t, "POST", "/v1/operations", `{"integration":"test-integration","operationId":"write","input":{}}`)

	resp, body := s.do(t, "GET", "/v1/audit-events", "")
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	events := body["events"].([]any)
	if len(events) == 0 {
		t.Fatal("expected audit events")
	}
	newest := events[0].(map[string]any)
	if newest["action"] != "operation.execute" || newest["outcome"] != "succeeded" || newest["actor"] != "dev-user" {
		t.Errorf("unexpected newest event: %v", newest)
	}

	noAudit := newServer(t, "operation.execute")
	resp, _ = noAudit.do(t, "GET", "/v1/audit-events", "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 without audit.read, got %d", resp.StatusCode)
	}
}

func TestUnimplementedRoutesSayso(t *testing.T) {
	s := newServer(t, allPerms...)
	for _, r := range []struct{ method, path string }{
		{"GET", "/v1/connections"},
		{"POST", "/v1/connections"},
		{"POST", "/v1/approvals/x/approve"},
	} {
		resp, body := s.do(t, r.method, r.path, "")
		if resp.StatusCode != http.StatusNotImplemented || errorCode(body) != "not_implemented" {
			t.Errorf("%s %s: expected 501 not_implemented, got %d %v", r.method, r.path, resp.StatusCode, body)
		}
	}
}

func TestRequestIDSanitized(t *testing.T) {
	s := newServer(t, allPerms...)
	resp, _ := s.do(t, "GET", "/v1/integrations", "", RequestIDHeader, "bad id <script>")
	if got := resp.Header.Get(RequestIDHeader); !strings.HasPrefix(got, "req_") {
		t.Fatalf("unsafe client request ID must be replaced, got %q", got)
	}
}
