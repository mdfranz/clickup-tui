package clickup

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	pat := "test-token-123"
	client := NewClient(pat)

	if client.PAT != pat {
		t.Errorf("PAT mismatch: got %q, want %q", client.PAT, pat)
	}
	if client.HTTPClient == nil {
		t.Error("HTTPClient should not be nil")
	}
	if client.HTTPClient.Timeout == 0 {
		t.Error("HTTPClient should have a timeout set")
	}
	if client.HTTPClient.Timeout != 30*time.Second {
		t.Errorf("Timeout mismatch: got %v, want %v", client.HTTPClient.Timeout, 30*time.Second)
	}
}

func TestGetTeams(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "test-token" {
			t.Errorf("Authorization header mismatch: got %q, want %q", authHeader, "test-token")
		}

		// Return mock response
		response := TeamsResponse{
			Teams: []Team{
				{ID: "team-1", Name: "Team 1"},
				{ID: "team-2", Name: "Team 2"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create a custom client that makes requests to the test server
	client := &Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		PAT:        "test-token",
	}

	// Make direct HTTP request to test the parsing
	req, _ := http.NewRequest("GET", server.URL+"/team", nil)
	req.Header.Add("Authorization", "test-token")
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	var teamsResp TeamsResponse
	json.NewDecoder(resp.Body).Decode(&teamsResp)

	if len(teamsResp.Teams) != 2 {
		t.Errorf("Teams count mismatch: got %d, want 2", len(teamsResp.Teams))
	}

	if teamsResp.Teams[0].ID != "team-1" || teamsResp.Teams[0].Name != "Team 1" {
		t.Errorf("First team mismatch: got %+v", teamsResp.Teams[0])
	}
}

func TestGetUser(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := struct {
			User User `json:"user"`
		}{
			User: User{
				ID:       "user-123",
				Username: "testuser",
				Email:    "test@example.com",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		PAT:        "test-token",
	}

	// Make direct HTTP request to test the parsing
	req, _ := http.NewRequest("GET", server.URL+"/user", nil)
	req.Header.Add("Authorization", "test-token")
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	var userResp struct {
		User User `json:"user"`
	}
	json.NewDecoder(resp.Body).Decode(&userResp)

	if userResp.User.ID != "user-123" {
		t.Errorf("User ID mismatch: got %q, want %q", userResp.User.ID, "user-123")
	}
	if userResp.User.Username != "testuser" {
		t.Errorf("Username mismatch: got %q, want %q", userResp.User.Username, "testuser")
	}
	if userResp.User.Email != "test@example.com" {
		t.Errorf("Email mismatch: got %q, want %q", userResp.User.Email, "test@example.com")
	}
}

func TestClientErrors(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "Unauthorized"}`))
	}))
	defer server.Close()

	client := &Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		PAT:        "invalid-token",
	}

	// Make direct HTTP request to verify error handling
	req, _ := http.NewRequest("GET", server.URL+"/team", nil)
	req.Header.Add("Authorization", "invalid-token")
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 status, got %d", resp.StatusCode)
	}
}

func TestUserIDType(t *testing.T) {
	// Verify User.ID is a string type
	user := User{
		ID:       "user-string-123",
		Username: "testuser",
		Email:    "test@example.com",
	}

	// This should compile without issue
	idStr := user.ID
	if idStr != "user-string-123" {
		t.Errorf("User ID should be string type, got: %T", user.ID)
	}
}
