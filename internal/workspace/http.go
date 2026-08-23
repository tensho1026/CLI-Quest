package workspace

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func serveHTTP(workspace string, port int, mode string) error {
	mux := http.NewServeMux()
	switch mode {
	case "health":
		mux.HandleFunc("/health", func(response http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodGet {
				http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			_ = os.WriteFile(filepath.Join(workspace, ".health-checked"), []byte(time.Now().Format(time.RFC3339)+"\n"), 0o600)
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]string{"status": "ok"})
		})
	case "auth":
		mux.HandleFunc("/profile", func(response http.ResponseWriter, request *http.Request) {
			if request.Header.Get("Authorization") != "Bearer cliquest-secret" {
				http.Error(response, "Unauthorized", http.StatusUnauthorized)
				return
			}
			_ = os.WriteFile(filepath.Join(workspace, ".auth-succeeded"), []byte(time.Now().Format(time.RFC3339)+"\n"), 0o600)
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]string{"user": "engineer", "role": "admin"})
		})
	case "json":
		mux.HandleFunc("/users", func(response http.ResponseWriter, request *http.Request) {
			_ = os.WriteFile(filepath.Join(workspace, ".json-requested"), []byte(time.Now().Format(time.RFC3339)+"\n"), 0o600)
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]any{"users": []map[string]any{
				{"id": 1, "name": "Aki", "role": "viewer"},
				{"id": 2, "name": "Mina", "role": "admin"},
				{"id": 3, "name": "Ren", "role": "editor"},
			}})
		})
	case "debug":
		mux.HandleFunc("/legacy", func(response http.ResponseWriter, request *http.Request) {
			response.Header().Set("Location", "/v2")
			response.WriteHeader(http.StatusMovedPermanently)
		})
		mux.HandleFunc("/v2", func(response http.ResponseWriter, request *http.Request) {
			_ = os.WriteFile(filepath.Join(workspace, ".debug-followed"), []byte(time.Now().Format(time.RFC3339)+"\n"), 0o600)
			response.Header().Set("X-API-Version", "2")
			_, _ = io.WriteString(response, "API v2 ready\n")
		})
	case "incident":
		mux.HandleFunc("/health", func(response http.ResponseWriter, request *http.Request) {
			config, _ := os.ReadFile(filepath.Join(workspace, "service.conf"))
			if !strings.Contains(string(config), "DATABASE_URL=postgres://db.internal/app") {
				http.Error(response, "database configuration invalid", http.StatusInternalServerError)
				return
			}
			_ = os.WriteFile(filepath.Join(workspace, ".incident-resolved"), []byte(time.Now().Format(time.RFC3339)+"\n"), 0o600)
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]string{"status": "ok", "database": "connected"})
		})
	default:
		return fmt.Errorf("unsupported HTTP scene mode %q", mode)
	}
	server := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return err
	}
	if err := markReady(workspace); err != nil {
		listener.Close()
		return err
	}
	return server.Serve(listener)
}
