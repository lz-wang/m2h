package server

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// workspaceChanged is the single change signal the server streams: any
// watched input — a single-file root or a directory tree — reports the same
// event, and the WebUI answers by refetching /api/files and reloading (or
// replacing) the open document.
const workspaceChanged = "workspace-changed"

type eventHub struct {
	mu            sync.Mutex
	clients       map[chan string]struct{}
	keepAlive     time.Duration
	clientChanged chan struct{}
}

func newEventHub(keepAlive time.Duration) *eventHub {
	return &eventHub{
		clients:       make(map[chan string]struct{}),
		keepAlive:     keepAlive,
		clientChanged: make(chan struct{}, 1),
	}
}

func (hub *eventHub) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", http.MethodGet)
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := response.(http.Flusher)
	if !ok {
		http.Error(response, "streaming is unsupported", http.StatusInternalServerError)
		return
	}

	response.Header().Set("Content-Type", "text/event-stream")
	response.Header().Set("Cache-Control", "no-cache")
	response.Header().Set("Connection", "keep-alive")
	response.Header().Set("X-Accel-Buffering", "no")

	client := make(chan string, 1)
	hub.add(client)
	defer hub.remove(client)

	_, _ = fmt.Fprint(response, ": connected\n\n")
	flusher.Flush()

	ticker := time.NewTicker(hub.keepAlive)
	defer ticker.Stop()
	for {
		select {
		case <-request.Context().Done():
			return
		case event := <-client:
			_, _ = fmt.Fprintf(response, "event: %s\ndata: {}\n\n", event)
			flusher.Flush()
		case <-ticker.C:
			_, _ = fmt.Fprint(response, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}

func (hub *eventHub) publish(event string) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	for client := range hub.clients {
		select {
		case client <- event:
		default:
		}
	}
}

func (hub *eventHub) add(client chan string) {
	hub.mu.Lock()
	hub.clients[client] = struct{}{}
	hub.mu.Unlock()
	hub.notifyClientChanged()
}

func (hub *eventHub) remove(client chan string) {
	hub.mu.Lock()
	delete(hub.clients, client)
	hub.mu.Unlock()
	hub.notifyClientChanged()
}

func (hub *eventHub) clientCount() int {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	return len(hub.clients)
}

func (hub *eventHub) notifyClientChanged() {
	select {
	case hub.clientChanged <- struct{}{}:
	default:
	}
}
