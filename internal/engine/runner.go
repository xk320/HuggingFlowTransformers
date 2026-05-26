package engine

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"

	"huggingflowtransformers/internal/supervisor"
)

type RunnerOptions struct {
	Path        string
	BaseURL     string
	InternalKey string
	NodeName    func(device int) string
	CompatMode  bool
	ArgvWrapper string
	RawLogDir   string
	RawLogHours int
}

type Runner struct {
	options RunnerOptions
	mu      sync.Mutex
	procs   map[int]*exec.Cmd
}

func NewRunner(options RunnerOptions) *Runner {
	return &Runner{options: options, procs: map[int]*exec.Cmd{}}
}

func (r *Runner) StartDevice(ctx context.Context, device int, lines chan<- supervisor.EngineLine) error {
	return r.start(ctx, device, lines)
}

func (r *Runner) RestartDevice(ctx context.Context, device int, lines chan<- supervisor.EngineLine) error {
	_ = r.StopDevice(device)
	return r.start(ctx, device, lines)
}

func (r *Runner) StopDevice(device int) error {
	r.mu.Lock()
	cmd := r.procs[device]
	delete(r.procs, device)
	r.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if pgid, err := syscall.Getpgid(cmd.Process.Pid); err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
		time.Sleep(2 * time.Second)
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
	if err := cmd.Process.Kill(); err != nil {
		return err
	}
	_, _ = cmd.Process.Wait()
	return nil
}

func (r *Runner) start(ctx context.Context, device int, lines chan<- supervisor.EngineLine) error {
	node := ""
	if r.options.NodeName != nil {
		node = r.options.NodeName(device)
	}
	options := Options{
		Path:        r.options.Path,
		BaseURL:     r.options.BaseURL,
		InternalKey: r.options.InternalKey,
		Node:        node,
		Device:      device,
		CompatMode:  r.options.CompatMode,
	}
	cmd := command(ctx, r.options.Path, options.Args())
	cmd.Env = wrappedEnvironment(r.options.ArgvWrapper)
	var stdout io.ReadCloser
	var stderr io.ReadCloser
	var ptyFile *os.File
	var err error
	if r.options.CompatMode {
		ptyFile, err = pty.Start(cmd)
		if err != nil {
			return fmt.Errorf("start HFT embedded runtime: %w", err)
		}
		stdout = ptyFile
	} else {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		stdout, err = cmd.StdoutPipe()
		if err != nil {
			return err
		}
		stderr, err = cmd.StderrPipe()
		if err != nil {
			return err
		}
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("start HFT embedded runtime: %w", err)
		}
	}
	if err := verifyProcessGuard(cmd.Process.Pid, []string{
		"--pool",
		"--address",
		"stratum",
		r.options.BaseURL,
		r.options.InternalKey,
	}); err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		if ptyFile != nil {
			_ = ptyFile.Close()
		}
		return err
	}
	raw := r.openRawLog(device)

	r.mu.Lock()
	r.procs[device] = cmd
	r.mu.Unlock()

	go scanLines(device, stdout, lines, raw)
	if stderr != nil {
		go scanLines(device, stderr, lines, raw)
	}
	go func() {
		defer func() {
			if ptyFile != nil {
				_ = ptyFile.Close()
			}
			if raw != nil {
				_ = raw.Close()
			}
		}()
		err := cmd.Wait()
		r.mu.Lock()
		wasCurrent := r.procs[device] == cmd
		if r.procs[device] == cmd {
			delete(r.procs, device)
		}
		r.mu.Unlock()
		if ctx.Err() == nil && wasCurrent {
			text := "runtime_worker_exit"
			if err != nil {
				text += ": " + err.Error()
			}
			lines <- supervisor.EngineLine{Device: device, Text: text}
		}
	}()
	return nil
}

func wrappedEnvironment(wrapperPath string) []string {
	env := os.Environ()
	if wrapperPath == "" {
		return env
	}
	return append(env,
		"LD_PRELOAD="+wrapperPath,
		"HFT_PROCESS_TITLE=HuggingFlowTransformers-runtime",
	)
}

func command(ctx context.Context, path string, args []string) *exec.Cmd {
	return exec.CommandContext(ctx, path, args...)
}

func verifyProcessGuard(pid int, forbidden []string) error {
	if goruntime.GOOS != "linux" || pid <= 0 {
		return nil
	}
	time.Sleep(20 * time.Millisecond)
	payload, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil || len(payload) == 0 {
		return nil
	}
	cmdline := strings.ReplaceAll(string(payload), "\x00", " ")
	for _, token := range forbidden {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if strings.Contains(cmdline, token) {
			return fmt.Errorf("HFT runtime process guard failed")
		}
	}
	return nil
}

func scanLines(device int, reader io.Reader, lines chan<- supervisor.EngineLine, raw *rawLog) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		text := scanner.Text()
		if raw != nil {
			_ = raw.WriteLine(text)
		}
		lines <- supervisor.EngineLine{Device: device, Text: text}
	}
}

func (r *Runner) openRawLog(device int) *rawLog {
	if r.options.RawLogDir == "" {
		return nil
	}
	if err := os.MkdirAll(r.options.RawLogDir, 0o700); err != nil {
		return nil
	}
	cleanupRawLogs(r.options.RawLogDir, r.options.RawLogHours)
	path := filepath.Join(r.options.RawLogDir, fmt.Sprintf("device-%d.raw.log", device))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil
	}
	return &rawLog{file: file}
}

func cleanupRawLogs(dir string, hours int) {
	if hours <= 0 {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, entry.Name()))
	}
}

type rawLog struct {
	mu   sync.Mutex
	file *os.File
}

func (r *rawLog) WriteLine(text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, err := fmt.Fprintln(r.file, text)
	return err
}

func (r *rawLog) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.file.Close()
}
