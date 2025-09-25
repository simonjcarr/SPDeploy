# SPDeploy

**Automatic Git deployments made simple.** SPDeploy monitors your Git repositories and automatically pulls changes to your servers - no complex CI/CD setup required. Perfect for developers who want instant deployments without the overhead.

✨ **Zero configuration** • 🚀 **Single binary** • 🔒 **SSH-based** • 📦 **Any Git provider** • ⚡ **Lightweight**

## Quick Start (Under 2 Minutes)

```bash
# 1. Download from GitHub releases page
# Visit: https://github.com/simonjcarr/spdeploy/releases/latest
# Download the binary for your platform and extract

# 2. Add your repository
spdeploy add git@github.com:username/myapp.git /var/www/myapp

# 3. Start monitoring
spdeploy run
```

That's it! SPDeploy now watches your repository and auto-deploys on every push.

## Features

- **Universal Git Support** - Works with GitHub, GitLab, BitBucket, and any Git server with SSH
- **Simple as It Gets** - No YAML configs, no pipelines, no complexity
- **Deploy Scripts** - Automatically runs `spdeploy.sh` after pulling (build, restart services, etc.)
- **Multiple Repositories** - Monitor unlimited repos from different providers simultaneously
- **Branch Control** - Deploy from any branch (main, develop, staging, etc.)
- **Lightweight** - Single 10MB binary, uses < 20MB RAM
- **Cross-Platform** - Linux, macOS, Windows, ARM/AMD64
- **Background Service** - Runs as a daemon, survives reboots
- **Real-time Logs** - Monitor deployments as they happen

## Installation

Download the appropriate binary for your platform from the [GitHub releases page](https://github.com/simonjcarr/spdeploy/releases/latest).

### Linux/macOS

```bash
# Option 1: Download via command line (replace with your platform)
# Linux AMD64
curl -L https://github.com/simonjcarr/spdeploy/releases/latest/download/spdeploy-linux-amd64.tar.gz | tar -xz
sudo mv spdeploy /usr/local/bin/

# Linux ARM64
curl -L https://github.com/simonjcarr/spdeploy/releases/latest/download/spdeploy-linux-arm64.tar.gz | tar -xz
sudo mv spdeploy /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/simonjcarr/spdeploy/releases/latest/download/spdeploy-darwin-amd64.tar.gz | tar -xz
sudo mv spdeploy /usr/local/bin/

# macOS (Apple Silicon)
curl -L https://github.com/simonjcarr/spdeploy/releases/latest/download/spdeploy-darwin-arm64.tar.gz | tar -xz
sudo mv spdeploy /usr/local/bin/

# Option 2: Manual download
# 1. Go to https://github.com/simonjcarr/spdeploy/releases/latest
# 2. Download the .tar.gz file for your platform
# 3. Extract and install:
tar -xzf spdeploy-*.tar.gz
chmod +x spdeploy
sudo mv spdeploy /usr/local/bin/
```

### Windows

1. Visit the [releases page](https://github.com/simonjcarr/spdeploy/releases/latest)
2. Download the Windows ZIP file for your architecture (amd64)
3. Extract the ZIP file
4. Add the directory to your PATH environment variable

## SSH Setup

SPDeploy uses SSH keys for secure authentication. If you don't have SSH access set up:

```bash
# Generate SSH key (if needed)
ssh-keygen -t ed25519 -C "spdeploy@server"

# Copy public key to clipboard
cat ~/.ssh/id_ed25519.pub

# Add to your Git provider:
# GitHub: Settings → SSH Keys → New SSH key
# GitLab: Settings → SSH Keys → Add key
# BitBucket: Personal settings → SSH keys → Add key

# Test connection
ssh -T git@github.com
```

## Usage Guide

### Basic Commands

```bash
# Add a repository
spdeploy add <ssh-url> <deploy-path> [options]
  --branch <name>   # Branch to monitor (default: main)
  --script <path>   # Custom deploy script

# Examples
spdeploy add git@github.com:team/webapp.git /var/www/webapp
spdeploy add git@gitlab.com:api/v2.git /opt/api --branch develop
spdeploy add git@bitbucket.org:company/app.git ~/app --script deploy.sh

# Start/stop monitoring
spdeploy run         # Start in foreground
spdeploy run -d      # Start as daemon (background)
spdeploy stop        # Stop daemon
spdeploy status      # Check if daemon is running

# View repositories
spdeploy list

# Remove repository
spdeploy remove <ssh-url>

# View logs
spdeploy log         # Show all logs
spdeploy log -f      # Follow logs (real-time)
```

### Deploy Scripts

Create a deploy script in your repository that runs automatically after each pull. By default, SPDeploy looks for `spdeploy.sh` in your repository root, but you can specify a custom script path using the `--script` flag.

**Important:** The script is executed from the repository's root directory, not from the directory where the script is located. All relative paths in your script will be relative to the repository root.

```bash
#!/bin/bash
# spdeploy.sh - Runs automatically after pulling changes
# Working directory: repository root

# Node.js example
npm ci --production
npm run build
pm2 restart app

# Python example
pip install -r requirements.txt
systemctl restart myapp

# Docker example
docker-compose down
docker-compose up -d --build
```

If your script is in a subdirectory and needs to reference files relative to its location:

```bash
#!/bin/bash
# scripts/deploy.sh - Script in subdirectory
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Files created here will be in the repository root
echo "Deployed at $(date)" > deploy.log

# To create files in the script's directory
echo "Deployed at $(date)" > "$SCRIPT_DIR/deploy.log"
```

Make it executable:
```bash
chmod +x spdeploy.sh
git add spdeploy.sh
git commit -m "Add deploy script"
git push
```

### Running as a Service

**Linux (systemd)**
```bash
# Create service file
sudo tee /etc/systemd/system/spdeploy.service > /dev/null << EOF
[Unit]
Description=SPDeploy Git Auto-Deploy
After=network.target

[Service]
Type=simple
User=$USER
ExecStart=/usr/local/bin/spdeploy run
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable spdeploy
sudo systemctl start spdeploy
```

**macOS (launchd)**
```bash
# Create plist file
sudo tee ~/Library/LaunchAgents/io.spdeploy.plist > /dev/null << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>io.spdeploy</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/spdeploy</string>
        <string>run</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
EOF

# Load service
launchctl load ~/Library/LaunchAgents/io.spdeploy.plist
```

## Advanced Features

### Multiple Repositories

Monitor different projects from various Git providers:

```bash
# Frontend from GitHub
spdeploy add git@github.com:company/frontend.git /var/www/frontend

# API from GitLab
spdeploy add git@gitlab.com:backend/api.git /var/www/api --branch production

# Microservice from BitBucket
spdeploy add git@bitbucket.org:team/service.git /opt/service --branch develop

# Start monitoring all
spdeploy run -d
```

### Branch-based Deployments

Deploy different branches to different locations:

```bash
# Production
spdeploy add git@github.com:app/website.git /var/www/prod --branch main

# Staging
spdeploy add git@github.com:app/website.git /var/www/staging --branch staging

# Development
spdeploy add git@github.com:app/website.git /var/www/dev --branch develop
```

### Custom Scripts

Use different scripts for different deployments:

```bash
# Production with extensive checks
spdeploy add git@github.com:app/api.git /opt/api \
  --branch main \
  --script scripts/deploy-production.sh

# Staging with quick deploy
spdeploy add git@github.com:app/api.git /opt/api-staging \
  --branch staging \
  --script scripts/deploy-staging.sh
```

### Docker Deployments

SPDeploy works great with Docker:

```bash
# spdeploy.sh in your repo
#!/bin/bash
docker-compose pull
docker-compose up -d --remove-orphans
docker image prune -f
```

### Private Git Servers

SPDeploy works with any Git server that supports SSH:

```bash
# Self-hosted GitLab
spdeploy add git@gitlab.company.com:internal/app.git /var/www/app

# Gitea
spdeploy add git@git.company.com:team/project.git /opt/project

# GitHub Enterprise
spdeploy add git@github.enterprise.com:org/repo.git /var/www/repo
```

## Configuration

SPDeploy stores its configuration in:
- **Linux/macOS**: `~/.spdeploy/config.json`
- **Windows**: `%USERPROFILE%\.spdeploy\config.json`

You can edit this file directly if needed, but using CLI commands is recommended.

## Troubleshooting

### Repository not updating?

```bash
# Check if SPDeploy is running
ps aux | grep spdeploy

# View logs for errors
spdeploy log -f

# Test SSH connection
ssh -T git@github.com

# Manually test repository access
git ls-remote git@github.com:username/repo.git
```

### Permission denied errors?

```bash
# Ensure SSH key has correct permissions
chmod 600 ~/.ssh/id_ed25519
chmod 644 ~/.ssh/id_ed25519.pub

# Add SSH key to agent
ssh-add ~/.ssh/id_ed25519

# Check repository path permissions
ls -la /var/www/myapp
```

### Deploy script not running?

```bash
# Check script is executable
ls -la spdeploy.sh  # Should show -rwxr-xr-x

# Make executable
chmod +x spdeploy.sh

# Test script manually
cd /var/www/myapp && ./spdeploy.sh
```

## Performance

- **CPU**: Minimal usage, polls every 60 seconds
- **Memory**: < 20MB RAM per instance
- **Disk**: 10MB binary + your repository sizes
- **Network**: Only active during git pull operations

## Security Best Practices

1. **Use deploy keys** instead of personal SSH keys
2. **Restrict repository access** to read-only where possible
3. **Run as non-root user** for better security
4. **Use separate SSH keys** for different environments
5. **Monitor logs** regularly for unexpected activity
6. **Keep SPDeploy updated** for security patches

## Build from Source

```bash
# Clone repository
git clone https://github.com/simonjcarr/spdeploy.git
cd spdeploy

# Build
go build -o spdeploy cmd/spdeploy/main.go

# Install
sudo mv spdeploy /usr/local/bin/
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/simonjcarr/spdeploy/issues)
- **Discussions**: [GitHub Discussions](https://github.com/simonjcarr/spdeploy/discussions)
- **Email**: support@spdeploy.io

---

Made with ❤️ by developers, for developers. Because deployment should be simple.