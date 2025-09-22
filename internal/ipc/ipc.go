package ipc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"spdeploy/internal/config"
)

type Command struct {
	Type      string                 `json:"type"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

type Response struct {
	Success   bool                   `json:"success"`
	Data      map[string]interface{} `json:"data"`
	Error     string                 `json:"error,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

type IPCClient struct {
	commandDir  string
	responseDir string
}

func NewIPCClient() *IPCClient {
	cfg := config.NewConfig()
	baseDir := filepath.Dir(cfg.GetConfigPath())

	return &IPCClient{
		commandDir:  filepath.Join(baseDir, "commands"),
		responseDir: filepath.Join(baseDir, "responses"),
	}
}

func (c *IPCClient) SendCommand(cmdType string, data map[string]interface{}) (*Response, error) {
	// Ensure directories exist
	if err := os.MkdirAll(c.commandDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create command directory: %w", err)
	}
	if err := os.MkdirAll(c.responseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create response directory: %w", err)
	}

	// Create command
	cmd := Command{
		Type:      cmdType,
		Data:      data,
		Timestamp: time.Now(),
	}

	// Generate unique command ID
	cmdID := fmt.Sprintf("%d", time.Now().UnixNano())

	// Write command file
	cmdFile := filepath.Join(c.commandDir, fmt.Sprintf("%s.json", cmdID))
	cmdData, err := json.MarshalIndent(cmd, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}

	if err := os.WriteFile(cmdFile, cmdData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write command file: %w", err)
	}

	// Wait for response
	responseFile := filepath.Join(c.responseDir, fmt.Sprintf("%s.json", cmdID))
	timeout := 30 * time.Second
	start := time.Now()

	for {
		if time.Since(start) > timeout {
			// Clean up command file
			os.Remove(cmdFile)
			return nil, fmt.Errorf("command timed out")
		}

		if _, err := os.Stat(responseFile); err == nil {
			// Response file exists, read it
			responseData, err := os.ReadFile(responseFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read response file: %w", err)
			}

			var response Response
			if err := json.Unmarshal(responseData, &response); err != nil {
				return nil, fmt.Errorf("failed to unmarshal response: %w", err)
			}

			// Clean up files
			os.Remove(cmdFile)
			os.Remove(responseFile)

			return &response, nil
		}

		time.Sleep(100 * time.Millisecond)
	}
}

type IPCServer struct {
	commandDir  string
	responseDir string
	handlers    map[string]func(map[string]interface{}) (*Response, error)
}

func NewIPCServer() *IPCServer {
	cfg := config.NewConfig()
	baseDir := filepath.Dir(cfg.GetConfigPath())

	server := &IPCServer{
		commandDir:  filepath.Join(baseDir, "commands"),
		responseDir: filepath.Join(baseDir, "responses"),
		handlers:    make(map[string]func(map[string]interface{}) (*Response, error)),
	}

	// Ensure directories exist
	os.MkdirAll(server.commandDir, 0755)
	os.MkdirAll(server.responseDir, 0755)

	return server
}

func (s *IPCServer) RegisterHandler(cmdType string, handler func(map[string]interface{}) (*Response, error)) {
	s.handlers[cmdType] = handler
}

func (s *IPCServer) Start() error {
	go s.processCommands()
	return nil
}

func (s *IPCServer) processCommands() {
	for {
		// Check for command files
		files, err := os.ReadDir(s.commandDir)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		for _, file := range files {
			if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
				cmdFile := filepath.Join(s.commandDir, file.Name())
				s.processCommand(cmdFile)
			}
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func (s *IPCServer) processCommand(cmdFile string) {
	// Read command file
	data, err := os.ReadFile(cmdFile)
	if err != nil {
		return
	}

	var cmd Command
	if err := json.Unmarshal(data, &cmd); err != nil {
		return
	}

	// Find handler
	handler, exists := s.handlers[cmd.Type]
	if !exists {
		// Unknown command type
		response := &Response{
			Success:   false,
			Error:     fmt.Sprintf("unknown command type: %s", cmd.Type),
			Timestamp: time.Now(),
		}
		s.writeResponse(cmdFile, response)
		return
	}

	// Execute handler
	response, err := handler(cmd.Data)
	if err != nil {
		response = &Response{
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
		}
	}

	if response == nil {
		response = &Response{
			Success:   true,
			Timestamp: time.Now(),
		}
	}

	s.writeResponse(cmdFile, response)
}

func (s *IPCServer) writeResponse(cmdFile string, response *Response) {
	// Generate response file name
	cmdID := filepath.Base(cmdFile)
	cmdID = cmdID[:len(cmdID)-len(filepath.Ext(cmdID))] // Remove extension

	responseFile := filepath.Join(s.responseDir, fmt.Sprintf("%s.json", cmdID))

	// Write response
	responseData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(responseFile, responseData, 0644)

	// Remove command file
	os.Remove(cmdFile)
}