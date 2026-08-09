package server

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEventHubBroadcastsKeepsAliveAndCleansUp(t *testing.T) {
	t.Parallel()

	hub := newEventHub(20 * time.Millisecond)
	service := httptest.NewServer(hub)
	t.Cleanup(service.Close)

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, service.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := service.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(response.Body)
	line, err := reader.ReadString('\n')
	if err != nil || line != ": connected\n" {
		t.Fatalf("initial SSE line = %q, %v", line, err)
	}
	waitForClientCount(t, hub, 1)

	hub.publish(documentChanged)
	stream := readUntil(t, reader, "data: {}", time.Second)
	if !strings.Contains(stream, "event: document-changed") {
		t.Fatalf("SSE stream is missing event name: %q", stream)
	}
	stream = readUntil(t, reader, ": keep-alive", time.Second)
	if !strings.Contains(stream, ": keep-alive") {
		t.Fatalf("SSE stream is missing keep-alive: %q", stream)
	}

	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	waitForClientCount(t, hub, 0)
}

func TestEventHubRejectsMethodsAndMissingFlusher(t *testing.T) {
	t.Parallel()

	hub := newEventHub(time.Second)
	response := httptest.NewRecorder()
	hub.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", nil))
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("POST response = %d, Allow=%q", response.Code, response.Header().Get("Allow"))
	}

	plain := &plainResponse{header: make(http.Header)}
	hub.ServeHTTP(plain, httptest.NewRequest(http.MethodGet, "/", nil))
	if plain.status != http.StatusInternalServerError {
		t.Fatalf("non-streaming response status = %d", plain.status)
	}
}

type plainResponse struct {
	header http.Header
	status int
}

func (response *plainResponse) Header() http.Header { return response.header }

func (response *plainResponse) Write(body []byte) (int, error) {
	return len(body), nil
}

func (response *plainResponse) WriteHeader(status int) { response.status = status }

func waitForClientCount(t *testing.T, hub *eventHub, want int) {
	t.Helper()
	deadline := time.After(time.Second)
	for hub.clientCount() != want {
		select {
		case <-hub.clientChanged:
		case <-deadline:
			t.Fatalf("client count = %d, want %d", hub.clientCount(), want)
		}
	}
}

func readUntil(t *testing.T, reader *bufio.Reader, want string, timeout time.Duration) string {
	t.Helper()
	result := make(chan string, 1)
	go func() {
		var stream strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				result <- stream.String()
				return
			}
			stream.WriteString(line)
			if strings.Contains(stream.String(), want) {
				result <- stream.String()
				return
			}
		}
	}()
	select {
	case stream := <-result:
		return stream
	case <-time.After(timeout):
		t.Fatalf("SSE stream did not contain %q", want)
		return ""
	}
}
