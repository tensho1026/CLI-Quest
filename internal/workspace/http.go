package workspace

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
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
