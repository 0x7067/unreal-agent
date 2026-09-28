package messagesapi

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"uuid"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/primitives"
)

const DefaultMaxAttempts = primitives.DefaultRemoteMaxAttempts

type APIError struct {
	StatusCode int
	Type       string
	Message    string
	RequestID  string
}

func (err *APIError) Error() string {
	if err.Type != "" {
		return fmt.Sprintf("messages API error %s: %s", err.Type, err.Message)
	}
	return fmt.Sprintf("messages API request failed with status %d: %s", err.StatusCode, err.Message)
}

type Exchange struct {
	RequestBody  []byte
	StatusCode   int
	ResponseBody []byte
}

type Config struct {
	Endpoint string
	Headers  map[string][]string
	// Nil uses DefaultMaxAttempts.
	MaxAttempts *int
	// Trace borrows read-only bodies: request JSON and assembled response JSON or provider error.
	Trace func(Exchange)
}

type adapter struct {
	remote      *primitives.RemoteClient
	endpoint    string
	headers     http.Header
	maxAttempts int
	trace       func(Exchange)
}

var _ llm.Adapter = (*adapter)(nil)

func NewAdapter(remote *primitives.RemoteClient, config Config) (llm.Adapter, error) {
	if remote == nil {
		return nil, errors.New("remote client must be set")
	}
	if strings.TrimSpace(config.Endpoint) == "" {
		return nil, errors.New("messages API endpoint must be set")
	}
	maxAttempts := DefaultMaxAttempts
	if config.MaxAttempts != nil {
		maxAttempts = *config.MaxAttempts
	}
	if maxAttempts <= 0 {
		return nil, errors.New("max attempts must be positive")
	}
	headers := make(http.Header, len(config.Headers)+1)
	for name, values := range config.Headers {
		for _, value := range values {
			headers.Add(name, value)
		}
	}
	headers.Set("Accept", "text/event-stream")
	return &adapter{remote: remote, endpoint: config.Endpoint, headers: headers, maxAttempts: maxAttempts, trace: config.Trace}, nil
}

func (adapter *adapter) Respond(ctx context.Context, request llm.Request, _ llm.RequestOptions) (llm.Response, error) {
	body, err := requestBody(request)
	if err != nil {
		return llm.Response{}, err
	}
	result := adapter.exchange(ctx, body)
	if adapter.trace != nil && len(result.body) != 0 {
		adapter.trace(Exchange{RequestBody: body, StatusCode: result.status, ResponseBody: result.body})
	}
	if result.err != nil {
		return llm.Response{}, result.err
	}
	return decodeResponse(result.body)
}

func (adapter *adapter) remoteRequest(body []byte) primitives.RemoteRequest {
	request := primitives.DefaultRemoteRequest("llm.messagesapi", primitives.CorrelationID(uuid.New().String()), adapter.endpoint)
	request.Method = http.MethodPost
	request.Headers = adapter.headers.Clone()
	request.Body = body
	request.ResponseIdleTimeout = 30 * time.Minute
	request.RetryPolicy.MaxAttempts = 1
	request.SSE = &primitives.RemoteSSEOptions{MaxFrameSize: 256 << 20, FrameDelimiter: primitives.SSEFrameDelimiterStrip}
	return request
}

func providerError(status int, body []byte, headers http.Header) *APIError {
	var envelope struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
		RequestID string `json:"request_id"`
	}
	result := &APIError{StatusCode: status, RequestID: headers.Get("Request-Id")}
	if err := json.Unmarshal(body, &envelope); err == nil {
		result.Type, result.Message = envelope.Error.Type, envelope.Error.Message
		if envelope.RequestID != "" {
			result.RequestID = envelope.RequestID
		}
	}
	if result.Message == "" {
		result.Message = strings.TrimSpace(string(body))
		if result.Message == "" {
			result.Message = http.StatusText(status)
		}
	}
	return result
}

func remoteFailureError(ctx context.Context, event primitives.PrimitiveEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	failure, ok := event.Result.(primitives.PrimitiveFailureResult)
	if !ok {
		return errors.New("remote request failed with an invalid result")
	}
	return errors.New(failure.Error)
}
