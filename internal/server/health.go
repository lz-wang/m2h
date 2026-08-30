package server

import (
	"fmt"
	"net/http"
)

// serveHealth answers exactly one question: is the HTTP process initialized
// and accepting requests. It deliberately touches no documents — a git pull
// renaming files under the root must never flip a healthy process into a
// restarting one — and there is no database or upstream dependency to probe,
// so one endpoint suffices (no /livez + /readyz split). GET and HEAD reply
// "ok"; every other method is refused.
func (handler *documentHandler) serveHealth(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.Header().Set("Allow", "GET, HEAD")
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	if request.Method == http.MethodGet {
		_, _ = fmt.Fprintln(response, "ok")
	}
}
