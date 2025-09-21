# GoCD - Effortless Continuous Deployment

GoCD is a lightweight, cross-platform continuous deployment service that automatically syncs your code from GitHub to your development, staging, or production servers.

## 🚀 Quick Start

### 1. Download and Install

Download the latest release for your platform from the [releases page](https://github.com/your-username/gocd/releases).

```bash
# Make the binary executable (Unix/Mac)
chmod +x gocd

# Install GoCD to your system
./gocd install
```

### 2. Add a Repository

```bash
# Monitor a repository for pushes to main branch
gocd add --repo https://github.com/user/repo --branch main

# Monitor for pull request merges
gocd add --repo https://github.com/user/api --branch production --trigger pr

# Monitor for both pushes and PRs with custom path
gocd add --repo https://github.com/user/webapp --branch main --path ~/sites/webapp --trigger both
```

### 3. Start Monitoring

```bash
# Start the background service
gocd start

# Check status
gocd status

# View real-time logs
gocd --log -f
```

## 📋 Commands

### Repository Management
- `gocd add` - Add a repository to monitor
- `gocd list` - List all monitored repositories
- `gocd status` - Show service status and repository info

### Service Control
- `gocd start` - Start the monitoring service
- `gocd stop` - Stop the monitoring service
- `gocd install` - Install GoCD to system PATH
- `gocd uninstall` - Remove GoCD from system

### Monitoring
- `gocd --log` - Show recent logs
- `gocd --log -f` - Follow logs in real-time

## ⚙️ Configuration

### Repository Options

- `--repo` - GitHub repository URL (required)
- `--branch` - Branch to monitor (default: main)
- `--path` - Local directory path (default: current directory)
- `--trigger` - When to deploy: `push`, `pr`, or `both` (default: push)

### Trigger Types

- **push**: Deploy when commits are pushed to the branch
- **pr**: Deploy when pull requests are merged to the branch
- **both**: Deploy on both pushes and PR merges

## 🔧 Post-Deploy Scripts

GoCD will automatically run deployment scripts after successfully pulling changes:

### Unix/Linux/macOS
Create a `gocd.sh` script in your repository root:

```bash
#!/bin/bash
# gocd.sh - Deployment script

echo "Deploying application..."

# Install dependencies
npm install

# Build the application
npm run build

# Restart the service
sudo systemctl restart myapp

echo "Deployment complete!"
```

### Windows
Create a `gocd.bat` script in your repository root:

```batch
@echo off
REM gocd.bat - Deployment script

echo Deploying application...

REM Install dependencies
npm install

REM Build the application
npm run build

REM Restart the service
net stop myapp
net start myapp

echo Deployment complete!
```

### Environment Variables

Your deployment scripts have access to these environment variables:

- `GOCD_REPO_PATH` - Path to the repository
- `GOCD_TIMESTAMP` - Deployment timestamp
- `GOCD_VERSION` - GoCD version
- `GOCD_GIT_BRANCH` - Current Git branch
- `GOCD_GIT_COMMIT` - Current Git commit hash
- `GOCD_GIT_REMOTE` - Git remote URL

## 📁 File Locations

### Linux
- **Config**: `/etc/gocd/config.yaml`
- **Logs**: `/var/log/gocd/`

### macOS
- **Config**: `/etc/gocd/config.yaml` (root) or `/usr/local/etc/gocd/config.yaml`
- **Logs**: `/var/log/gocd/` (root) or `/usr/local/var/log/gocd/`

### Windows
- **Config**: `C:\ProgramData\GoCD\config.yaml`
- **Logs**: `C:\ProgramData\GoCD\logs\`

## 🔒 GitHub Authentication

For private repositories or higher rate limits, set your GitHub token:

```bash
export GITHUB_TOKEN=your_personal_access_token
```

## 🛠️ Building from Source

### Prerequisites
- Go 1.21 or later
- Git

### Build
```bash
# Clone the repository
git clone https://github.com/your-username/gocd.git
cd gocd

# Build for all platforms (creates dist/ directory)
./build.sh

# Build for current platform only
./build.sh local

# Install locally (copies to /usr/local/bin)
./gocd install
```

### Cross-Platform Build
Running `./build.sh` creates binaries for all supported platforms in the `dist/` directory:

```bash
# Build for all platforms (default behavior)
./build.sh

# Creates:
# dist/linux-amd64/gocd
# dist/linux-arm64/gocd
# dist/darwin-amd64/gocd (Intel Mac)
# dist/darwin-arm64/gocd (Apple Silicon)
# dist/windows-amd64/gocd.exe

# Create release packages (zip/tar.gz)
./build.sh package

# Set custom version
VERSION=1.2.3 ./build.sh

# Other commands
./build.sh help    # Show all options
./build.sh test    # Run tests
./build.sh fmt     # Format code
```

### Manual Go Build Commands
```bash
# Install dependencies
go mod download

# Build CLI binary
go build -o gocd ./cmd/gocd

# Build service binary
go build -o gocd-service ./cmd/gocd-service

# Cross-compile examples
GOOS=linux GOARCH=amd64 go build -o gocd-linux-amd64 ./cmd/gocd
GOOS=windows GOARCH=amd64 go build -o gocd-windows-amd64.exe ./cmd/gocd
```

### Development
```bash
# Run tests
go test ./...

# Format code
go fmt ./...

# Run linter (if golangci-lint is installed)
golangci-lint run

# Check for vulnerabilities
go list -json -deps ./... | nancy sleuth
```

## 📊 Example Workflows

### Local Development
```bash
# Monitor team changes in your local environment
gocd add --repo https://github.com/company/app --branch develop --path ~/dev/app
gocd start
```

### Staging Server
```bash
# Deploy every PR for testing
gocd add --repo https://github.com/company/app --branch main --trigger pr --path /var/www/staging
gocd start
```

### Production Server
```bash
# Deploy on merge to production branch
gocd add --repo https://github.com/company/app --branch production --path /var/www/production
gocd start
```

## 🐛 Troubleshooting

### Service won't start
```bash
# Check if already running
gocd status

# Check logs for errors
gocd --log

# Stop and restart
gocd stop
gocd start
```

### Repository not updating
```bash
# Check repository status
gocd list

# Verify GitHub connectivity
# Check logs for API errors
gocd --log -f
```

### Permission errors
```bash
# Ensure deployment scripts are executable
chmod +x gocd.sh

# Check repository permissions
ls -la /path/to/repo
```

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) for CLI
- Uses [go-git](https://github.com/go-git/go-git) for Git operations
- GitHub API integration via [go-github](https://github.com/google/go-github)
- Structured logging with [Zap](https://go.uber.org/zap)