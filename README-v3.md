# SPDeploy v3 - Simplified SSH-Only Version

A dramatically simplified version of SPDeploy that monitors git repositories and automatically pulls changes using SSH authentication only.

## Key Changes from v2

- **SSH-only**: No HTTPS URLs, no tokens, no OAuth - just SSH keys
- **92% less code**: Reduced from 7,280 lines to 576 lines
- **Pure git commands**: No GitHub/GitLab APIs, just `git fetch` and `git pull`
- **Single process**: No complex daemon management, just a simple loop
- **Simple config**: One JSON file with minimal options

## Features

✅ Monitor multiple git repositories
✅ Automatic pull on changes
✅ SSH key authentication
✅ Post-pull scripts
✅ Simple logging
✅ Lightweight (~576 lines of Go)

## Installation

```bash
# Build
go build -o spdeploy cmd/spdeploy/main.go

# Install to system
sudo cp spdeploy /usr/local/bin/
```

## Usage

### Add a repository
```bash
spdeploy add git@github.com:user/repo.git ~/projects/repo --branch main
```

### List repositories
```bash
spdeploy list
```

### Start monitoring
```bash
# Foreground
spdeploy run

# Background
spdeploy run -d
```

### Remove a repository
```bash
spdeploy remove git@github.com:user/repo.git
```

## Configuration

Config is stored in `~/.config/spdeploy/config.json`:

```json
{
  "check_interval": 60,
  "repositories": [
    {
      "url": "git@github.com:user/repo.git",
      "branch": "main",
      "path": "/home/user/projects/repo",
      "post_pull_script": "./deploy.sh"
    }
  ]
}
```

## Service Installation

### macOS (launchd)
```bash
cp services/com.spdeploy.plist ~/Library/LaunchAgents/
launchctl load ~/Library/LaunchAgents/com.spdeploy.plist
```

### Linux (systemd)
```bash
cp services/spdeploy.service ~/.config/systemd/user/
systemctl --user enable spdeploy
systemctl --user start spdeploy
```

## SSH Setup

Ensure you have SSH keys configured:

```bash
# Generate SSH key if needed
ssh-keygen -t ed25519 -C "your-email@example.com"

# Add to SSH agent
ssh-add ~/.ssh/id_ed25519

# Add public key to GitHub/GitLab
cat ~/.ssh/id_ed25519.pub
```

## Environment Variables

- `SPDEPLOY_DEBUG=1` - Enable debug logging
- `SPDEPLOY_QUIET=1` - Suppress console output (log file only)

## Architecture

```
cmd/spdeploy/
  main.go         (217 lines - CLI)
internal/
  config.go       (69 lines - JSON config)
  monitor.go      (115 lines - Main loop)
  git.go          (100 lines - Git operations)
  logger.go       (75 lines - Logging)

Total: 576 lines
```

## Why v3?

The v2 codebase grew too complex with:
- Multiple git provider APIs (GitHub, GitLab, Bitbucket)
- Token management systems
- OAuth flows
- Complex daemon/service management
- Provider abstraction layers

v3 simplifies to the core functionality:
- SSH-only (standard git authentication)
- Pure git commands (no APIs)
- Single process (OS handles restarts)
- Minimal configuration

This makes it more reliable, easier to debug, and simpler to maintain.