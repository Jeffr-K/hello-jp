package llm

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestOllamaProviderParsesResponse(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/chat" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(`{"message":{"role":"assistant","content":"運動"}}`)), Header: make(http.Header)}, nil
	})}
	provider, err := NewOllamaProvider(OllamaProviderConfig{Model: "qwen3:4b", BaseURL: "http://ollama.test", Client: client})
	if err != nil {
		t.Fatal(err)
	}
	response, err := provider.Complete(context.Background(), Request{Messages: []Message{{Role: RoleUser, Content: "운동을 일본어로"}}})
	if err != nil {
		t.Fatal(err)
	}
	if response.Message.Content != "運動" {
		t.Fatalf("unexpected content: %q", response.Message.Content)
	}
}
