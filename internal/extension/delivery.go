package extension

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/event"
	"github.com/beanruntime/bean/internal/field"
)

const (
	APIVersion              = "bean.extension/v1alpha1"
	TopicPrefix             = "bean.extension/"
	BearerTokensEnvironment = "BEAN_EXTENSION_BEARER_TOKENS"

	FailureUnavailable    = "extension_unavailable"
	FailureTimeout        = "extension_timeout"
	FailureAuthentication = "extension_authentication"
	FailureResponse       = "extension_response"
	FailureContract       = "extension_contract"
	FailureRedirect       = "extension_redirect"
)

func ParseBearerTokens(raw string) (map[string]string, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]string{}, nil
	}
	values := map[string]string{}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&values); err != nil {
		return nil, fmt.Errorf("%s must be a JSON object of Extension names to bearer tokens", BearerTokensEnvironment)
	}
	if values == nil {
		return nil, fmt.Errorf("%s must be a JSON object of Extension names to bearer tokens", BearerTokensEnvironment)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("%s must contain one JSON object", BearerTokensEnvironment)
	}
	for name, token := range values {
		if name == "" || strings.TrimSpace(token) == "" {
			return nil, fmt.Errorf("%s entries require non-empty names and tokens", BearerTokensEnvironment)
		}
	}
	return values, nil
}

type Invocation struct {
	APIVersion     string         `json:"apiVersion"`
	Extension      string         `json:"extension"`
	InvocationID   string         `json:"invocationId"`
	IdempotencyKey string         `json:"idempotencyKey"`
	Input          map[string]any `json:"input"`
}

type intent struct {
	Invocation
	Contract appir.Extension `json:"contract"`
}

type Provider interface {
	Call(context.Context, appir.Extension, Invocation) (map[string]any, error)
}

type DeliveryFailure struct {
	Code     string
	CanRetry bool
}

func (e *DeliveryFailure) Error() string   { return e.Code }
func (e *DeliveryFailure) Retryable() bool { return e.CanRetry }

func Enqueue(ctx context.Context, tx dbal.Transaction, definition appir.Extension, input map[string]any, invocationID string, createdAt time.Time) error {
	if invocationID == "" {
		return fmt.Errorf("Extension invocation ID is required")
	}
	if err := ValidateValues(definition.Input, input); err != nil {
		return err
	}
	invocation := Invocation{APIVersion: APIVersion, Extension: definition.Name, InvocationID: invocationID, IdempotencyKey: invocationID, Input: input}
	_, err := event.Enqueue(ctx, tx, TopicPrefix+definition.Name, intent{Invocation: invocation, Contract: definition}, event.Options{
		ID: invocationID, RetryDelay: time.Duration(definition.Retry.DelaySeconds) * time.Second,
		MaxAttempts: definition.Retry.MaxAttempts, CreatedAt: createdAt,
	})
	return err
}

func IsTopic(topic string) bool { return strings.HasPrefix(topic, TopicPrefix) }

func Deliver(ctx context.Context, provider Provider, topic string, payload map[string]any) error {
	name := strings.TrimPrefix(topic, TopicPrefix)
	if name == topic || name == "" {
		return &DeliveryFailure{Code: FailureContract}
	}
	if provider == nil {
		return &DeliveryFailure{Code: FailureUnavailable, CanRetry: true}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return &DeliveryFailure{Code: FailureContract}
	}
	var stored intent
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	if err = decoder.Decode(&stored); err != nil || stored.APIVersion != APIVersion || stored.Extension != name || stored.InvocationID == "" || stored.IdempotencyKey != stored.InvocationID || stored.Contract.Name != name {
		return &DeliveryFailure{Code: FailureContract}
	}
	invocation := stored.Invocation
	definition := stored.Contract
	if err = ValidateValues(definition.Input, invocation.Input); err != nil {
		return &DeliveryFailure{Code: FailureContract}
	}
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(definition.TimeoutSeconds)*time.Second)
	defer cancel()
	output, err := provider.Call(callCtx, definition, invocation)
	if callCtx.Err() != nil {
		return &DeliveryFailure{Code: FailureTimeout, CanRetry: true}
	}
	if err != nil {
		return err
	}
	if err = ValidateValues(definition.Output, output); err != nil {
		return &DeliveryFailure{Code: FailureResponse}
	}
	return nil
}

func ValidateValues(definitions map[string]appir.Field, values map[string]any) error {
	for name := range values {
		if _, exists := definitions[name]; !exists {
			return fmt.Errorf("undeclared Extension field %s", name)
		}
	}
	for name, definition := range definitions {
		value, exists := values[name]
		if !exists {
			value = nil
		}
		if definition.Type == "integer" && value != nil && !validInteger(value) {
			return fmt.Errorf("%s has invalid integer value", definition.Name)
		}
		if err := field.Validate(definition, value); err != nil {
			return err
		}
	}
	return nil
}

func validInteger(value any) bool {
	switch typed := value.(type) {
	case int, int64:
		return true
	case float64:
		return !math.IsNaN(typed) && !math.IsInf(typed, 0) && math.Trunc(typed) == typed && typed >= -9223372036854775808.0 && typed < 9223372036854775808.0
	case json.Number:
		_, err := typed.Int64()
		return err == nil
	default:
		return false
	}
}

type HTTPProvider struct {
	client *http.Client
	bearer map[string]string
}

func NewHTTPProvider(transport http.RoundTripper, bearer map[string]string) *HTTPProvider {
	if transport == nil {
		transport = http.DefaultTransport
	}
	credentials := make(map[string]string, len(bearer))
	for name, token := range bearer {
		credentials[name] = token
	}
	return &HTTPProvider{client: &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errRedirect
		},
	}, bearer: credentials}
}

var errRedirect = errors.New("Extension redirect refused")

func (p *HTTPProvider) Call(ctx context.Context, definition appir.Extension, invocation Invocation) (map[string]any, error) {
	encoded, err := json.Marshal(invocation)
	if err != nil {
		return nil, &DeliveryFailure{Code: FailureContract}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, definition.Endpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, &DeliveryFailure{Code: FailureContract}
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Idempotency-Key", invocation.IdempotencyKey)
	if definition.Authentication == AuthenticationBearer {
		token := p.bearer[definition.Name]
		if token == "" {
			return nil, &DeliveryFailure{Code: FailureAuthentication}
		}
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := p.client.Do(request)
	if err != nil {
		if errors.Is(err, errRedirect) {
			return nil, &DeliveryFailure{Code: FailureRedirect}
		}
		return nil, &DeliveryFailure{Code: FailureUnavailable, CanRetry: true}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		retryable := response.StatusCode == http.StatusRequestTimeout || response.StatusCode == http.StatusTooEarly || response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500
		return nil, &DeliveryFailure{Code: FailureResponse, CanRetry: retryable}
	}
	limited := io.LimitReader(response.Body, MaxResponseBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil || len(body) > MaxResponseBytes {
		return nil, &DeliveryFailure{Code: FailureResponse}
	}
	var envelope struct {
		Output map[string]any `json:"output"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err = decoder.Decode(&envelope); err != nil || envelope.Output == nil {
		return nil, &DeliveryFailure{Code: FailureResponse}
	}
	if err = decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, &DeliveryFailure{Code: FailureResponse}
	}
	return envelope.Output, nil
}
