package internal

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestGetPIDFilePath(t *testing.T) {
	// Set HOME to a known value
	oldHome := os.Getenv("HOME")
	testHome := "/test/home"
	os.Setenv("HOME", testHome)
	defer os.Setenv("HOME", oldHome)

	expected := filepath.Join(testHome, ".spdeploy", "spdeploy.pid")
	actual := getPIDFilePath()

	if actual != expected {
		t.Errorf("Expected PID file path to be '%s', got '%s'", expected, actual)
	}
}

func TestWriteDaemonPID(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "spdeploy-pid-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Write PID
	testPID := 12345
	err = WriteDaemonPID(testPID)
	if err != nil {
		t.Fatalf("Failed to write PID: %v", err)
	}

	// Verify PID file exists and contains correct value
	pidFile := filepath.Join(tmpDir, ".spdeploy", "spdeploy.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("Failed to read PID file: %v", err)
	}

	writtenPID, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatalf("Failed to parse PID: %v", err)
	}

	if writtenPID != testPID {
		t.Errorf("Expected PID %d, got %d", testPID, writtenPID)
	}
}

func TestReadDaemonPID(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "spdeploy-pid-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	t.Run("NoPIDFile", func(t *testing.T) {
		pid, err := readDaemonPID()
		if err == nil {
			t.Error("Expected error when PID file doesn't exist")
		}
		if pid != 0 {
			t.Errorf("Expected PID to be 0 when file doesn't exist, got %d", pid)
		}
	})

	t.Run("ValidPIDFile", func(t *testing.T) {
		// Write a PID file
		pidDir := filepath.Join(tmpDir, ".spdeploy")
		os.MkdirAll(pidDir, 0755)
		pidFile := filepath.Join(pidDir, "spdeploy.pid")

		testPID := 54321
		err := os.WriteFile(pidFile, []byte(strconv.Itoa(testPID)), 0644)
		if err != nil {
			t.Fatalf("Failed to write test PID file: %v", err)
		}

		pid, err := readDaemonPID()
		if err != nil {
			t.Errorf("Failed to read PID: %v", err)
		}
		if pid != testPID {
			t.Errorf("Expected PID %d, got %d", testPID, pid)
		}
	})

	t.Run("InvalidPIDFile", func(t *testing.T) {
		// Write invalid content to PID file
		pidDir := filepath.Join(tmpDir, ".spdeploy")
		os.MkdirAll(pidDir, 0755)
		pidFile := filepath.Join(pidDir, "spdeploy.pid")

		err := os.WriteFile(pidFile, []byte("not-a-number"), 0644)
		if err != nil {
			t.Fatalf("Failed to write test PID file: %v", err)
		}

		pid, err := readDaemonPID()
		if err == nil {
			t.Error("Expected error for invalid PID file content")
		}
		if pid != 0 {
			t.Errorf("Expected PID to be 0 for invalid content, got %d", pid)
		}
	})
}

func TestIsDaemonRunning(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "spdeploy-pid-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	t.Run("NoPIDFile", func(t *testing.T) {
		if IsDaemonRunning() {
			t.Error("IsDaemonRunning should return false when PID file doesn't exist")
		}
	})

	t.Run("CurrentProcess", func(t *testing.T) {
		// Write current process PID
		currentPID := os.Getpid()
		err := WriteDaemonPID(currentPID)
		if err != nil {
			t.Fatalf("Failed to write PID: %v", err)
		}

		if !IsDaemonRunning() {
			t.Error("IsDaemonRunning should return true for current process")
		}
	})

	t.Run("NonExistentProcess", func(t *testing.T) {
		// Write a PID that definitely doesn't exist
		// Using a very high PID number that's unlikely to be in use
		err := WriteDaemonPID(99999999)
		if err != nil {
			t.Fatalf("Failed to write PID: %v", err)
		}

		if IsDaemonRunning() {
			t.Error("IsDaemonRunning should return false for non-existent process")
		}
	})
}

func TestCleanupDaemonPID(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "spdeploy-pid-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	t.Run("CleanupOwnPID", func(t *testing.T) {
		// Write current process PID
		currentPID := os.Getpid()
		err := WriteDaemonPID(currentPID)
		if err != nil {
			t.Fatalf("Failed to write PID: %v", err)
		}

		// Cleanup should remove the file
		CleanupDaemonPID()

		pidFile := filepath.Join(tmpDir, ".spdeploy", "spdeploy.pid")
		if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
			t.Error("PID file should have been removed")
		}
	})

	t.Run("DontCleanupOtherPID", func(t *testing.T) {
		// Write a different PID
		otherPID := os.Getpid() + 1
		err := WriteDaemonPID(otherPID)
		if err != nil {
			t.Fatalf("Failed to write PID: %v", err)
		}

		// Cleanup should NOT remove the file
		CleanupDaemonPID()

		pidFile := filepath.Join(tmpDir, ".spdeploy", "spdeploy.pid")
		if _, err := os.Stat(pidFile); os.IsNotExist(err) {
			t.Error("PID file should NOT have been removed for different PID")
		}
	})

	t.Run("NoPIDFile", func(t *testing.T) {
		// Ensure no PID file exists
		pidFile := filepath.Join(tmpDir, ".spdeploy", "spdeploy.pid")
		os.Remove(pidFile)

		// Should not panic or error
		CleanupDaemonPID()
	})
}