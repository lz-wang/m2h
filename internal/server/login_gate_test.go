package server

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginGateLimitsWorkAndRate(t *testing.T) {
	gate := newLoginGate()
	for i := 0; i < 4; i++ {
		if !gate.acquire() {
			t.Fatal("slot refused")
		}
	}
	if gate.acquire() {
		t.Fatal("exceeded concurrency limit")
	}
	for i := 0; i < 4; i++ {
		gate.release()
	}
	for i := 4; i < 60; i++ {
		if !gate.acquire() {
			t.Fatal("rate limit reached early")
		}
		gate.release()
	}
	if gate.acquire() {
		t.Fatal("exceeded rate limit")
	}
	gate.attempts = []time.Time{time.Now().Add(-time.Minute)}
	if !gate.acquire() {
		t.Fatal("expired attempt blocked login")
	}
	gate.release()
}

func TestOriginRequiresSchemeAndConfiguredPublicOrigin(t *testing.T) {
	request := httptest.NewRequest("POST", "http://docs.example.com/api/auth/login", nil)
	handler := &documentHandler{auth: &authStack{publicOrigin: "https://docs.example.com"}}
	if !handler.originAllowed(request, "https://docs.example.com") || handler.originAllowed(request, "http://docs.example.com") {
		t.Fatal("configured origin was not enforced")
	}
	handler.auth.publicOrigin = ""
	if !handler.originAllowed(request, "http://docs.example.com") || handler.originAllowed(request, "https://docs.example.com") {
		t.Fatal("direct origin scheme was not enforced")
	}
}
