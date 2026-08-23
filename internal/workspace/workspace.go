package workspace

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tensho1026/CLI-Quest/internal/scene"
)

type Manager struct {
	root       string
	executable string
}

func New(root string) (*Manager, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("find cliquest executable: %w", err)
	}
	return &Manager{root: filepath.Join(root, "workspaces"), executable: executable}, nil
}

func (m *Manager) Path(sceneID string) string {
	return filepath.Join(m.root, sceneID)
}

func (m *Manager) Remove(sceneID string) error {
	path := m.Path(sceneID)
	if filepath.Dir(path) != m.root {
		return fmt.Errorf("refusing to remove unsafe workspace path %q", path)
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove workspace: %w", err)
	}
	return nil
}

func (m *Manager) Setup(def scene.Definition) (map[string]string, error) {
	if err := m.Remove(def.ID); err != nil {
		return nil, err
	}
	workspace := m.Path(def.ID)
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	if err := createFiles(workspace, def.Setup.Files, m.executable); err != nil {
		m.Remove(def.ID)
		return nil, err
	}

	resources := map[string]string{}
	var err error
	switch def.Setup.Type {
	case "filesystem":
		// Files above are the complete setup.
	case "git_wrong_branch":
		err = setupGitWrongBranch(workspace, resources)
	case "git_restore_file":
		err = setupGitRestoreFile(workspace)
	case "git_lost_commit":
		err = setupGitLostCommit(workspace)
	case "port_conflict":
		err = m.startHelper(workspace, resources, "__hold-port", strconv.Itoa(def.Setup.Port))
	case "runaway_process":
		err = m.startHelper(workspace, resources, "__runaway")
	case "http_server":
		err = m.startHelper(workspace, resources, "__http-server", strconv.Itoa(def.Setup.Port), def.Setup.Mode)
	default:
		err = fmt.Errorf("unsupported setup type %q", def.Setup.Type)
	}
	if err != nil {
		Cleanup(resources)
		m.Remove(def.ID)
		return nil, err
	}
	return resources, nil
}

func createFiles(root string, specs []scene.FileSpec, executable string) error {
	for _, spec := range specs {
		path, err := safeJoin(root, spec.Path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		mode := os.FileMode(spec.Mode)
		if mode == 0 {
			mode = 0o600
		}
		content := strings.ReplaceAll(spec.Content, "{{CLIQUEST}}", shellQuote(executable))
		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
		if err != nil {
			return fmt.Errorf("create %s: %w", spec.Path, err)
		}
		if content != "" {
			if _, err := io.WriteString(file, content); err != nil {
				file.Close()
				return err
			}
		}
		if spec.Size > int64(len(content)) {
			remaining := spec.Size - int64(len(content))
			block := make([]byte, 64*1024)
			for remaining > 0 {
				part := int64(len(block))
				if part > remaining {
					part = remaining
				}
				if _, err := file.Write(block[:part]); err != nil {
					file.Close()
					return err
				}
				remaining -= part
			}
		}
		if err := file.Close(); err != nil {
			return err
		}
		if err := os.Chmod(path, mode); err != nil {
			return err
		}
	}
	return nil
}

func safeJoin(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) {
		return "", fmt.Errorf("unsafe scene path %q", relative)
	}
	clean := filepath.Clean(relative)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe scene path %q", relative)
	}
	return filepath.Join(root, clean), nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func initGit(dir string) error {
	if _, err := runGit(dir, "init", "-b", "main"); err != nil {
		return err
	}
	if _, err := runGit(dir, "config", "user.name", "CLI Quest"); err != nil {
		return err
	}
	_, err := runGit(dir, "config", "user.email", "cliquest@example.invalid")
	return err
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

func setupGitWrongBranch(dir string, resources map[string]string) error {
	if err := initGit(dir); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, "README.md"), "# Sample application\n\nStable release.\n"); err != nil {
		return err
	}
	if _, err := runGit(dir, "add", "README.md"); err != nil {
		return err
	}
	if _, err := runGit(dir, "commit", "-m", "initial release"); err != nil {
		return err
	}
	baseline, err := runGit(dir, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	resources["baseline_commit"] = baseline
	if err := writeFile(filepath.Join(dir, "feature.txt"), "authentication feature\n"); err != nil {
		return err
	}
	if _, err := runGit(dir, "add", "feature.txt"); err != nil {
		return err
	}
	_, err = runGit(dir, "commit", "-m", "add authentication feature (wrong branch)")
	return err
}

func setupGitRestoreFile(dir string) error {
	if err := initGit(dir); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, "config.ini"), "environment=production\nretries=3\n"); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, "notes.txt"), "Do not discard unrelated files.\n"); err != nil {
		return err
	}
	if _, err := runGit(dir, "add", "."); err != nil {
		return err
	}
	if _, err := runGit(dir, "commit", "-m", "add production config"); err != nil {
		return err
	}
	return writeFile(filepath.Join(dir, "config.ini"), "environment=development\nretries=0\nBROKEN=true\n")
}

func setupGitLostCommit(dir string) error {
	if err := initGit(dir); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, "README.md"), "Recovery practice repository.\n"); err != nil {
		return err
	}
	if _, err := runGit(dir, "add", "README.md"); err != nil {
		return err
	}
	if _, err := runGit(dir, "commit", "-m", "initial commit"); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, "recovery-note.txt"), "CLIQUEST_RECOVERED_7F3A\n"); err != nil {
		return err
	}
	if _, err := runGit(dir, "add", "recovery-note.txt"); err != nil {
		return err
	}
	if _, err := runGit(dir, "commit", "-m", "important recovery note"); err != nil {
		return err
	}
	_, err := runGit(dir, "reset", "--hard", "HEAD~1")
	return err
}

func (m *Manager) startHelper(workspace string, resources map[string]string, helper string, args ...string) error {
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return err
	}
	token := hex.EncodeToString(tokenBytes)
	ready := filepath.Join(workspace, ".cliquest-ready")
	logPath := filepath.Join(workspace, "service.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	commandArgs := []string{helper, workspace, token}
	commandArgs = append(commandArgs, args...)
	cmd := exec.Command(m.executable, commandArgs...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start scene helper: %w", err)
	}
	logFile.Close()
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	resources["pid"] = strconv.Itoa(cmd.Process.Pid)
	resources["token"] = token
	resources["helper"] = helper
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		if _, err := os.Stat(ready); err == nil {
			return nil
		}
		select {
		case <-wait:
			logData, _ := os.ReadFile(logPath)
			return fmt.Errorf("scene helper stopped before it was ready: %s", strings.TrimSpace(string(logData)))
		default:
		}
		time.Sleep(50 * time.Millisecond)
	}
	Cleanup(resources)
	return fmt.Errorf("scene helper did not become ready; see %s", logPath)
}

func Cleanup(resources map[string]string) {
	pid, err := strconv.Atoi(resources["pid"])
	if err != nil || pid <= 1 || resources["token"] == "" {
		return
	}
	if !processCommandContains(pid, resources["token"]) {
		return
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = process.Signal(syscall.SIGTERM)
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		if !processAlive(pid) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	if processCommandContains(pid, resources["token"]) {
		_ = process.Kill()
	}
}

func processCommandContains(pid int, token string) bool {
	output, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	return err == nil && bytes.Contains(output, []byte(token))
}

func processAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, os.ErrPermission)
}

// RunHelper handles an internal helper process. It returns true when args named
// an internal command, so callers can keep these commands out of public help.
func RunHelper(args []string) (bool, error) {
	if len(args) == 0 || !strings.HasPrefix(args[0], "__") {
		return false, nil
	}
	if len(args) < 3 {
		return true, fmt.Errorf("invalid internal helper arguments")
	}
	workspace, token := args[1], args[2]
	switch args[0] {
	case "__hold-port":
		if len(args) != 4 {
			return true, fmt.Errorf("invalid port helper arguments")
		}
		port, err := strconv.Atoi(args[3])
		if err != nil {
			return true, err
		}
		return true, holdPort(workspace, port)
	case "__runaway":
		return true, runAway(workspace)
	case "__http-server":
		if len(args) != 5 {
			return true, fmt.Errorf("invalid HTTP helper arguments")
		}
		port, err := strconv.Atoi(args[3])
		if err != nil {
			return true, err
		}
		return true, serveHTTP(workspace, port, args[4])
	default:
		return true, fmt.Errorf("unknown internal helper %q (token %s)", args[0], token[:min(4, len(token))])
	}
}

func markReady(workspace string) error {
	return os.WriteFile(filepath.Join(workspace, ".cliquest-ready"), []byte("ready\n"), 0o600)
}

func holdPort(workspace string, port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}
	defer listener.Close()
	if err := markReady(workspace); err != nil {
		return err
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		_, _ = io.WriteString(conn, "CLI Quest port conflict process\n")
		_ = conn.Close()
	}
}

func runAway(workspace string) error {
	if err := markReady(workspace); err != nil {
		return err
	}
	var value uint64 = 1
	for {
		started := time.Now()
		for time.Since(started) < 150*time.Millisecond {
			value = value*1664525 + 1013904223
		}
		if value == 0 {
			fmt.Print("")
		}
		time.Sleep(600 * time.Millisecond)
	}
}
