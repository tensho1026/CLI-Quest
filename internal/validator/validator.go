package validator

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
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
	result := checkRule(def.Validation, def, active)
	if result.Clear && result.Message == "" {
		result.Message = def.Success
	}
	return result
}

func checkRule(rule scene.Validation, def scene.Definition, active *progress.ActiveScene) Result {
	switch rule.Type {
	case "all":
		for _, nested := range rule.Validators {
			result := checkRule(nested, def, active)
			if !result.Clear {
				return result
			}
		}
		return Result{Clear: true}
	case "any":
		var messages []string
		for _, nested := range rule.Validators {
			result := checkRule(nested, def, active)
			if result.Clear {
				return Result{Clear: true}
			}
			if result.Message != "" {
				messages = append(messages, result.Message)
			}
		}
		return Result{Message: strings.Join(messages, " ")}
	case "executable":
		path, err := safePath(active.Workspace, rule.Path)
		if err != nil {
			return Result{Message: fmt.Sprintf("%s is missing or unsafe.", rule.Path)}
		}
		info, err := os.Stat(path)
		if err != nil {
			return Result{Message: fmt.Sprintf("%s does not exist.", rule.Path)}
		}
		if info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			return Result{Clear: true}
		}
		return Result{Message: fmt.Sprintf("%s is still not executable.", rule.Path)}
	case "file_exists":
		path, err := safePath(active.Workspace, rule.Path)
		if err == nil {
			if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
				return Result{Clear: true}
			}
		}
		return Result{Message: fmt.Sprintf("%s does not exist yet.", rule.Path)}
	case "file_mode":
		path, err := safePath(active.Workspace, rule.Path)
		if err == nil {
			if info, statErr := os.Stat(path); statErr == nil && fmt.Sprintf("%04o", info.Mode().Perm()) == rule.Value {
				return Result{Clear: true}
			}
		}
		return Result{Message: fmt.Sprintf("%s does not have mode %s yet.", rule.Path, rule.Value)}
	case "file_contains":
		path, err := safePath(active.Workspace, rule.Path)
		if err == nil {
			data, readErr := os.ReadFile(path)
			if readErr == nil && bytes.Contains(data, []byte(rule.Value)) {
				return Result{Clear: true}
			}
		}
		return Result{Message: fmt.Sprintf("%s does not contain the required answer yet.", rule.Path)}
	case "file_not_contains":
		path, err := safePath(active.Workspace, rule.Path)
		if err == nil {
			data, readErr := os.ReadFile(path)
			if readErr == nil && !bytes.Contains(data, []byte(rule.Value)) {
				return Result{Clear: true}
			}
		}
		return Result{Message: fmt.Sprintf("%s still contains the unwanted value.", rule.Path)}
	case "git_branch":
		if _, err := gitOutput(active.Workspace, "rev-parse", "--verify", "refs/heads/"+rule.Name); err == nil {
			return Result{Clear: true}
		}
		return Result{Message: fmt.Sprintf("Git branch %s does not exist.", rule.Name)}
	case "git_wrong_branch":
		return checkGitWrongBranch(def, active)
	case "git_clean":
		if err := safeWorkspaceRoot(active.Workspace); err != nil {
			return Result{Message: "The Git workspace path is unsafe."}
		}
		args := []string{"diff", "--quiet", "HEAD"}
		if rule.Path != "" {
			args = append(args, "--", rule.Path)
		}
		cmd := exec.Command("git", args...)
		cmd.Dir = active.Workspace
		if err := cmd.Run(); err == nil {
			return Result{Clear: true}
		}
		return Result{Message: fmt.Sprintf("%s still differs from the committed version.", rule.Path)}
	case "git_no_conflicts":
		output, err := gitOutput(active.Workspace, "ls-files", "-u")
		if err == nil && strings.TrimSpace(output) == "" {
			return Result{Clear: true}
		}
		return Result{Message: "The repository still has unresolved merge conflicts."}
	case "git_file_contains":
		content, err := gitOutput(active.Workspace, "show", "HEAD:"+rule.Path)
		if err == nil && strings.Contains(content, rule.Value) {
			return Result{Clear: true}
		}
		return Result{Message: fmt.Sprintf("HEAD does not contain the expected content in %s.", rule.Path)}
	case "git_commit_contains":
		copy := def
		copy.Validation = rule
		return checkReachableGitContent(copy, active)
	case "git_bisect_answer":
		path, err := safePath(active.Workspace, rule.Path)
		if err == nil {
			data, readErr := os.ReadFile(path)
			if readErr == nil && strings.Contains(string(data), active.Resources["culprit_commit"]) {
				return Result{Clear: true}
			}
		}
		return Result{Message: "The answer does not identify the first bad commit."}
	case "process_stopped":
		pid, err := strconv.Atoi(active.Resources["pid"])
		if err == nil && !processAlive(pid) {
			return Result{Clear: true}
		}
		return Result{Message: "The CLI Quest process is still running. Identify its PID and stop it."}
	case "port_available":
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", def.Setup.Port))
		if err == nil {
			listener.Close()
			return Result{Clear: true}
		}
		return Result{Message: fmt.Sprintf("Port %d is still in use.", def.Setup.Port)}
	case "marker_exists":
		path, err := safePath(active.Workspace, rule.Path)
		if err == nil {
			if _, statErr := os.Stat(path); statErr == nil {
				return Result{Clear: true}
			}
		}
		return Result{Message: "The server has not received the required request yet."}
	default:
		return Result{Message: fmt.Sprintf("Unsupported validator %q.", def.Validation.Type)}
	}
}

func safePath(workspace, relative string) (string, error) {
	root, err := filepath.Abs(workspace)
	if err != nil {
		return "", err
	}
	rootInfo, err := os.Lstat(root)
	if err != nil || rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return "", fmt.Errorf("unsafe workspace")
	}
	if filepath.IsAbs(relative) {
		return "", fmt.Errorf("absolute path")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	joined := filepath.Join(root, filepath.Clean(relative))
	if joined != root && !strings.HasPrefix(joined, root+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes workspace")
	}
	resolved, err := filepath.EvalSymlinks(joined)
	if err != nil {
		return "", err
	}
	if resolved != resolvedRoot && !strings.HasPrefix(resolved, resolvedRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("symlink escapes workspace")
	}
	return resolved, nil
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
	found, err := batchContains(active.Workspace, strings.Fields(commits), def.Validation.Path, def.Validation.Value)
	if err != nil {
		return Result{Message: "Could not inspect reachable Git content."}
	}
	if found {
		return Result{Clear: true, Message: def.Success}
	}
	return Result{Message: "The lost file is not present in any reachable commit yet."}
}

func batchContains(dir string, commits []string, path, value string) (bool, error) {
	if len(commits) == 0 {
		return false, nil
	}
	if err := safeWorkspaceRoot(dir); err != nil {
		return false, err
	}
	command := exec.Command("git", "cat-file", "--batch")
	command.Dir = dir
	input, err := command.StdinPipe()
	if err != nil {
		return false, err
	}
	output, err := command.StdoutPipe()
	if err != nil {
		return false, err
	}
	if err := command.Start(); err != nil {
		return false, err
	}
	writeDone := make(chan error, 1)
	go func() {
		for _, commit := range commits {
			if _, err := fmt.Fprintf(input, "%s:%s\n", commit, path); err != nil {
				writeDone <- err
				return
			}
		}
		writeDone <- input.Close()
	}()

	reader := bufio.NewReader(output)
	found := false
	for range commits {
		header, err := reader.ReadString('\n')
		if err != nil {
			_ = command.Process.Kill()
			_ = command.Wait()
			<-writeDone
			return false, err
		}
		trimmedHeader := strings.TrimSpace(header)
		if strings.HasSuffix(trimmedHeader, " missing") {
			continue
		}
		fields := strings.Fields(trimmedHeader)
		if len(fields) != 3 {
			_ = command.Process.Kill()
			_ = command.Wait()
			<-writeDone
			return false, fmt.Errorf("unexpected git cat-file response %q", strings.TrimSpace(header))
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil || size < 0 {
			_ = command.Process.Kill()
			_ = command.Wait()
			<-writeDone
			return false, fmt.Errorf("invalid git object size %q", fields[2])
		}
		content, err := io.ReadAll(io.LimitReader(reader, size))
		if err != nil || int64(len(content)) != size {
			_ = command.Process.Kill()
			_ = command.Wait()
			<-writeDone
			if err == nil {
				err = io.ErrUnexpectedEOF
			}
			return false, err
		}
		separator, err := reader.ReadByte()
		if err != nil || separator != '\n' {
			_ = command.Process.Kill()
			_ = command.Wait()
			<-writeDone
			if err == nil {
				err = fmt.Errorf("git cat-file response is missing separator")
			}
			return false, err
		}
		if fields[1] == "blob" && bytes.Contains(content, []byte(value)) {
			found = true
		}
	}
	if err := <-writeDone; err != nil {
		_ = command.Wait()
		return false, err
	}
	if err := command.Wait(); err != nil {
		return false, err
	}
	return found, nil
}

func gitOutput(dir string, args ...string) (string, error) {
	if err := safeWorkspaceRoot(dir); err != nil {
		return "", err
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func safeWorkspaceRoot(workspace string) error {
	root, err := filepath.Abs(workspace)
	if err != nil {
		return err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("workspace root is not a real directory")
	}
	return nil
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
