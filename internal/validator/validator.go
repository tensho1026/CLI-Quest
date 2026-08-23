package validator

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/tensho1026/CLI-Quest/internal/progress"
	"github.com/tensho1026/CLI-Quest/internal/scene"
)

type Result struct {
	Clear   bool
	Message string
}

func Check(def scene.Definition, active *progress.ActiveScene) Result {
	if active == nil || active.SceneID != def.ID {
		return Result{Message: "The active scene does not match this definition."}
	}
	switch def.Validation.Type {
	case "executable":
		info, err := os.Stat(filepath.Join(active.Workspace, def.Validation.Path))
		if err != nil {
			return Result{Message: fmt.Sprintf("%s does not exist.", def.Validation.Path)}
		}
		if info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			return Result{Clear: true, Message: def.Success}
		}
		return Result{Message: fmt.Sprintf("%s is still not executable.", def.Validation.Path)}
	case "file_contains":
		data, err := os.ReadFile(filepath.Join(active.Workspace, def.Validation.Path))
		if err == nil && bytes.Contains(data, []byte(def.Validation.Value)) {
			return Result{Clear: true, Message: def.Success}
		}
		return Result{Message: fmt.Sprintf("%s does not contain the required answer yet.", def.Validation.Path)}
	case "git_wrong_branch":
		return checkGitWrongBranch(def, active)
	case "git_clean":
		cmd := exec.Command("git", "diff", "--quiet", "HEAD", "--", def.Validation.Path)
		cmd.Dir = active.Workspace
		if err := cmd.Run(); err == nil {
			return Result{Clear: true, Message: def.Success}
		}
		return Result{Message: fmt.Sprintf("%s still differs from the committed version.", def.Validation.Path)}
	case "git_commit_contains":
		return checkReachableGitContent(def, active)
	case "process_stopped":
		pid, err := strconv.Atoi(active.Resources["pid"])
		if err == nil && !processAlive(pid) {
			return Result{Clear: true, Message: def.Success}
		}
		return Result{Message: "The CLI Quest process is still running. Identify its PID and stop it."}
	case "port_available":
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", def.Setup.Port))
		if err == nil {
			listener.Close()
			return Result{Clear: true, Message: def.Success}
		}
		return Result{Message: fmt.Sprintf("Port %d is still in use.", def.Setup.Port)}
	case "marker_exists":
		if _, err := os.Stat(filepath.Join(active.Workspace, def.Validation.Path)); err == nil {
			return Result{Clear: true, Message: def.Success}
		}
		return Result{Message: "The server has not received the required request yet."}
	default:
		return Result{Message: fmt.Sprintf("Unsupported validator %q.", def.Validation.Type)}
	}
}

func checkGitWrongBranch(def scene.Definition, active *progress.ActiveScene) Result {
	baseline := active.Resources["baseline_commit"]
	main, err := gitOutput(active.Workspace, "rev-parse", "refs/heads/main")
	if err != nil || strings.TrimSpace(main) != baseline {
		return Result{Message: "main has not been restored to its original commit."}
	}
	branches, err := gitOutput(active.Workspace, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return Result{Message: "Could not inspect the repository branches."}
	}
	for _, branch := range strings.Fields(branches) {
		if !strings.HasPrefix(branch, "feature/") {
			continue
		}
		content, err := gitOutput(active.Workspace, "show", branch+":feature.txt")
		if err == nil && strings.Contains(content, "authentication feature") {
			return Result{Clear: true, Message: def.Success}
		}
	}
	return Result{Message: "The authentication change is not reachable from a feature/* branch."}
}

func checkReachableGitContent(def scene.Definition, active *progress.ActiveScene) Result {
	commits, err := gitOutput(active.Workspace, "rev-list", "--all")
	if err != nil {
		return Result{Message: "Could not inspect reachable commits."}
	}
	for _, commit := range strings.Fields(commits) {
		content, err := gitOutput(active.Workspace, "show", commit+":"+def.Validation.Path)
		if err == nil && strings.Contains(content, def.Validation.Value) {
			return Result{Clear: true, Message: def.Success}
		}
	}
	return Result{Message: "The lost file is not present in any reachable commit yet."}
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func processAlive(pid int) bool {
	if pid <= 1 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}
