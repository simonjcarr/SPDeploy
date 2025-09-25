package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

func getPIDFilePath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".spdeploy", "spdeploy.pid")
}

func IsDaemonRunning() bool {
	pid, err := readDaemonPID()
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

func WriteDaemonPID(pid int) error {
	pidFile := getPIDFilePath()
	pidDir := filepath.Dir(pidFile)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		return fmt.Errorf("failed to create PID directory: %w", err)
	}

	// Write PID to file
	return os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644)
}

func readDaemonPID() (int, error) {
	pidFile := getPIDFilePath()
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, fmt.Errorf("invalid PID in file: %w", err)
	}

	return pid, nil
}

func ReadDaemonPID() (int, error) {
	return readDaemonPID()
}

func StopDaemon() error {
	pid, err := readDaemonPID()
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
	pidFile := getPIDFilePath()
	os.Remove(pidFile)

	return nil
}

func CleanupDaemonPID() {
	// Only cleanup if the PID file exists and it's our process
	pid, err := readDaemonPID()
	if err != nil {
		return
	}

	// Check if it's our PID
	if pid == os.Getpid() {
		pidFile := getPIDFilePath()
		os.Remove(pidFile)
	}
}