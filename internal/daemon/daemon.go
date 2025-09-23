package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"go.uber.org/zap"
	"spdeploy/internal/config"
	"spdeploy/internal/logger"
	"spdeploy/internal/monitor"
)

type Daemon struct {
	config  *config.ConfigManager
	monitor *monitor.Monitor
	pidFile string
}

func NewDaemon() *Daemon {
	cfg := config.NewConfig()
	pidFile := filepath.Join(filepath.Dir(cfg.GetConfigPath()), "spdeploy.pid")

	return &Daemon{
		config:  cfg,
		pidFile: pidFile,
	}
}

func (d *Daemon) Start() error {
	// Check if daemon is already running
	if d.IsRunning() {
		return fmt.Errorf("daemon is already running")
	}

	// Prevent running as root on Unix systems
	if os.Geteuid() == 0 {
		return fmt.Errorf("daemon should not be started as root - please run as a normal user")
	}

	// Start the daemon process
	return d.startDaemonProcess()
}

func (d *Daemon) Stop() error {
	pid, err := d.getPID()
	if err != nil {
		return fmt.Errorf("daemon is not running")
	}

	// Find the process
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find daemon process: %w", err)
	}

	// Send SIGTERM signal
	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("failed to stop daemon: %w", err)
	}

	// Remove PID file
	os.Remove(d.pidFile)

	logger.Info("Daemon stopped successfully")
	return nil
}

func (d *Daemon) IsRunning() bool {
	pid, err := d.getPID()
	if err != nil {
		return false
	}

	// Check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process is alive
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func (d *Daemon) startDaemonProcess() error {
	// Get the path to the current executable
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks to get the actual binary path
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	// Start the same binary with --service flag as a background process
	cmd := exec.Command(executable, "--service")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	// Set the process attributes to run as the current user
	cmd.SysProcAttr = getSysProcAttr()

	// Set environment to match current user
	cmd.Env = os.Environ()

	// Start the process
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start daemon process: %w", err)
	}

	// Write PID file
	err = d.writePID(cmd.Process.Pid)
	if err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Detach the process
	cmd.Process.Release()

	logger.Info("Daemon started successfully", zap.Int("pid", cmd.Process.Pid))
	return nil
}


func (d *Daemon) getPID() (int, error) {
	data, err := os.ReadFile(d.pidFile)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, fmt.Errorf("invalid PID in file: %w", err)
	}

	return pid, nil
}

func (d *Daemon) writePID(pid int) error {
	pidDir := filepath.Dir(d.pidFile)
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		return err
	}

	return os.WriteFile(d.pidFile, []byte(strconv.Itoa(pid)), 0644)
}

// Global functions for CLI access
func Start() error {
	daemon := NewDaemon()
	return daemon.Start()
}

func Stop() error {
	daemon := NewDaemon()
	return daemon.Stop()
}

func IsRunning() bool {
	daemon := NewDaemon()
	return daemon.IsRunning()
}