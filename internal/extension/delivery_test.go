package extension_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/extension"
)

func TestHTTPProviderSendsTypedVersionedAuthenticatedInvocation(t *testing.T) {
	var received extension.Invocation
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/json" || request.Header.Get("Accept") != "application/json" || request.Header.Get("Idempotency-Key") != "invocation-1" || request.Header.Get("Authorization") != "Bearer test-secret" {
			t.Errorf("request=%s headers=%v", request.Method, request.Header)
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"output":{"accepted":true}}`))
	}))
	defer server.Close()
	definition := providerDefinition(server.URL, "bearer")
	app := appir.Empty()
	app.Extensions[definition.Name] = definition
	payload := invocationPayload(t, extension.Invocation{APIVersion: extension.APIVersion, Extension: definition.Name, InvocationID: "invocation-1", IdempotencyKey: "invocation-1", Input: map[string]any{"message": "hello"}})
	provider := extension.NewHTTPProvider(nil, map[string]string{definition.Name: "test-secret"})
	if err := extension.Deliver(context.Background(), app, provider, extension.TopicPrefix+definition.Name, payload); err != nil {
		t.Fatal(err)
	}
	if received.APIVersion != extension.APIVersion || received.Extension != definition.Name || received.InvocationID != "invocation-1" || received.Input["message"] != "hello" {
		t.Fatalf("invocation=%+v", received)
	}
}

func TestHTTPProviderClassifiesSafeDeterministicFailures(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		retryable bool
	}{
		{"bad request", http.StatusBadRequest, false},
		{"request timeout", http.StatusRequestTimeout, true},
		{"too many requests", http.StatusTooManyRequests, true},
		{"server unavailable", http.StatusServiceUnavailable, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) { response.WriteHeader(test.status) }))
			defer server.Close()
			definition := providerDefinition(server.URL, "none")
			_, err := extension.NewHTTPProvider(nil, nil).Call(context.Background(), definition, testInvocation(definition.Name))
			assertDeliveryFailure(t, err, extension.FailureResponse, test.retryable)
		})
	}

	definition := providerDefinition("http://127.0.0.1:1/unavailable", "bearer")
	_, err := extension.NewHTTPProvider(nil, nil).Call(context.Background(), definition, testInvocation(definition.Name))
	assertDeliveryFailure(t, err, extension.FailureAuthentication, false)
	if strings.Contains(err.Error(), "Bearer") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("unsafe error=%v", err)
	}
}

func TestHTTPProviderRejectsRedirectInvalidAndOversizedResponses(t *testing.T) {
	redirect := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Redirect(response, request, "/elsewhere", http.StatusFound)
	}))
	defer redirect.Close()
	definition := providerDefinition(redirect.URL, "none")
	_, err := extension.NewHTTPProvider(nil, nil).Call(context.Background(), definition, testInvocation(definition.Name))
	assertDeliveryFailure(t, err, extension.FailureRedirect, false)

	for _, body := range []string{`{"output":{"accepted":"yes"}}`, `{"output":{"accepted":true},"extra":true}`, `{"output":{"accepted":true}} trailing`, strings.Repeat("x", extension.MaxResponseBytes+1)} {
		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) { _, _ = response.Write([]byte(body)) }))
		definition = providerDefinition(server.URL, "none")
		app := appir.Empty()
		app.Extensions[definition.Name] = definition
		payload := invocationPayload(t, testInvocation(definition.Name))
		err = extension.Deliver(context.Background(), app, extension.NewHTTPProvider(nil, nil), extension.TopicPrefix+definition.Name, payload)
		assertDeliveryFailure(t, err, extension.FailureResponse, false)
		server.Close()
	}
}

func TestHTTPProviderRejectsFractionalAndOutOfRangeIntegerOutput(t *testing.T) {
	for _, body := range []string{`{"output":{"sequence":1.5}}`, `{"output":{"sequence":9223372036854775808}}`} {
		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) { _, _ = response.Write([]byte(body)) }))
		definition := providerDefinition(server.URL, "none")
		definition.Output = map[string]appir.Field{"sequence": {Name: "sequence", Type: "integer", Required: true}}
		app := appir.Empty()
		app.Extensions[definition.Name] = definition
		err := extension.Deliver(context.Background(), app, extension.NewHTTPProvider(nil, nil), extension.TopicPrefix+definition.Name, invocationPayload(t, testInvocation(definition.Name)))
		assertDeliveryFailure(t, err, extension.FailureResponse, false)
		server.Close()
	}
}

func TestDeliveryPreservesIntegerInputPrecision(t *testing.T) {
	definition := providerDefinition("https://provider.example/notify", "none")
	definition.Input = map[string]appir.Field{"sequence": {Name: "sequence", Type: "integer", Required: true}}
	app := appir.Empty()
	app.Extensions[definition.Name] = definition
	invocation := testInvocation(definition.Name)
	invocation.Input = map[string]any{"sequence": int64(9007199254740993)}
	encoded, err := json.Marshal(invocation)
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{}
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.UseNumber()
	if err = decoder.Decode(&payload); err != nil {
		t.Fatal(err)
	}
	provider := providerFunc(func(_ context.Context, _ appir.Extension, delivered extension.Invocation) (map[string]any, error) {
		number, ok := delivered.Input["sequence"].(json.Number)
		if !ok || number.String() != "9007199254740993" {
			t.Fatalf("sequence=%v (%T)", delivered.Input["sequence"], delivered.Input["sequence"])
		}
		return map[string]any{"accepted": true}, nil
	})
	if err = extension.Deliver(context.Background(), app, provider, extension.TopicPrefix+definition.Name, payload); err != nil {
		t.Fatal(err)
	}
}

type providerFunc func(context.Context, appir.Extension, extension.Invocation) (map[string]any, error)

func (f providerFunc) Call(ctx context.Context, definition appir.Extension, invocation extension.Invocation) (map[string]any, error) {
	return f(ctx, definition, invocation)
}

func TestDeliveryEnforcesTimeoutAndTypedMockOutput(t *testing.T) {
	definition := providerDefinition("https://provider.example/notify", "none")
	definition.TimeoutSeconds = 1
	app := appir.Empty()
	app.Extensions[definition.Name] = definition
	payload := invocationPayload(t, testInvocation(definition.Name))
	blocking := providerFunc(func(ctx context.Context, _ appir.Extension, _ extension.Invocation) (map[string]any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	started := time.Now()
	err := extension.Deliver(context.Background(), app, blocking, extension.TopicPrefix+definition.Name, payload)
	assertDeliveryFailure(t, err, extension.FailureTimeout, true)
	if time.Since(started) > 2*time.Second {
		t.Fatalf("timeout took %s", time.Since(started))
	}
	invalid := providerFunc(func(context.Context, appir.Extension, extension.Invocation) (map[string]any, error) {
		return map[string]any{"accepted": "yes"}, nil
	})
	err = extension.Deliver(context.Background(), app, invalid, extension.TopicPrefix+definition.Name, payload)
	assertDeliveryFailure(t, err, extension.FailureResponse, false)
}

func TestBearerTokenHostConfigurationIsStrictAndDoesNotEchoSecrets(t *testing.T) {
	tokens, err := extension.ParseBearerTokens(`{"notify":"secret-value"}`)
	if err != nil || tokens["notify"] != "secret-value" {
		t.Fatalf("tokens=%v err=%v", tokens, err)
	}
	for _, invalid := range []string{`[]`, `{"notify":""}`, `{"notify":"secret"} trailing`} {
		if _, err = extension.ParseBearerTokens(invalid); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("invalid=%q err=%v", invalid, err)
		}
	}
}

func providerDefinition(endpoint, authentication string) appir.Extension {
	return appir.Extension{
		Name: "notify", Transport: "http", Endpoint: endpoint,
		Input:       map[string]appir.Field{"message": {Name: "message", Type: "string", Required: true}},
		Output:      map[string]appir.Field{"accepted": {Name: "accepted", Type: "boolean", Required: true}},
		Permissions: []string{"network"}, SideEffects: []string{"external_write"}, Authentication: authentication, TimeoutSeconds: 5,
		Retry: appir.ExtensionRetry{MaxAttempts: 3, DelaySeconds: 1}, Idempotency: "required", Transaction: "after_commit", Failure: "retry_then_fail",
	}
}

func testInvocation(name string) extension.Invocation {
	return extension.Invocation{APIVersion: extension.APIVersion, Extension: name, InvocationID: "invocation-1", IdempotencyKey: "invocation-1", Input: map[string]any{"message": "hello"}}
}

func invocationPayload(t *testing.T, invocation extension.Invocation) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(invocation)
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{}
	if err = json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func assertDeliveryFailure(t *testing.T, err error, code string, retryable bool) {
	t.Helper()
	classified, ok := err.(*extension.DeliveryFailure)
	if !ok || classified.Code != code || classified.Retryable() != retryable || err.Error() != code {
		t.Fatalf("err=%v (%T), want %s retryable=%v", err, err, code, retryable)
	}
}
