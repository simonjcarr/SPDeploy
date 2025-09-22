# SPDeploy - Effortless Continuous Deployment

SPDeploy is a lightweight, cross-platform continuous deployment service that automatically syncs your code from GitHub to your development, staging, or production servers.

## 🚀 Quick Start

### 1. Download and Install

#### Download Pre-built Binaries

Download the latest release for your platform from the [releases page](https://github.com/simonjcarr/spdeploy/releases).

**Linux (amd64)**
```bash
curl -L https://github.com/simonjcarr/spdeploy/releases/latest/download/spdeploy-linux-amd64.tar.gz | tar -xz
chmod +x spdeploy
sudo mv spdeploy /usr/local/bin/
```

**Linux (arm64)**
```bash
curl -L https://github.com/simonjcarr/spdeploy/releases/latest/download/spdeploy-linux-arm64.tar.gz | tar -xz
chmod +x spdeploy
sudo mv spdeploy /usr/local/bin/
```

**macOS (Intel)**
```bash
curl -L https://github.com/simonjcarr/spdeploy/releases/latest/download/spdeploy-darwin-amd64.tar.gz | tar -xz
chmod +x spdeploy
sudo mv spdeploy /usr/local/bin/
```

**macOS (Apple Silicon)**
```bash
curl -L https://github.com/simonjcarr/spdeploy/releases/latest/download/spdeploy-darwin-arm64.tar.gz | tar -xz
chmod +x spdeploy
sudo mv spdeploy /usr/local/bin/
```

**Windows**
```powershell
# Download the Windows ZIP from releases page
# Extract spdeploy.exe to a directory in your PATH
# Or add the extracted directory to your PATH environment variable
```

#### Install SPDeploy Service

After downloading the binary:

```bash
# Install SPDeploy as a user service
spdeploy install
```

This will:
- Copy the binary to the appropriate system location
- Set up a user-level service (systemd on Linux, LaunchAgent on macOS, Scheduled Task on Windows)
- Create necessary configuration directories

### 2. Set Up Repository Authentication

#### For SSH URLs (Recommended)
```bash
# Get SSH key setup instructions
spdeploy repo auth git@github.com:user/repo.git

# Follow the platform-specific instructions to set up SSH keys
```

#### For HTTPS URLs
```bash
# Interactive token setup
spdeploy repo auth https://github.com/user/repo.git

# Or use existing GitHub token
export GITHUB_TOKEN=your_personal_access_token
```

### 3. Add a Repository to Monitor

```bash
# Add a repository with required path
spdeploy repo add --repo https://github.com/user/repo --path ~/deployments/myapp

# Monitor a different branch
spdeploy repo add --repo https://github.com/user/api --branch production --path /var/www/api

# Monitor pull request merges only
spdeploy repo add --repo https://github.com/user/webapp --branch main --path ~/sites/webapp --trigger pr

# Monitor both pushes and PR merges
spdeploy repo add --repo https://github.com/user/app --branch develop --path ~/dev/app --trigger both
```

### 4. Start Monitoring

```bash
# Start the SPDeploy daemon
spdeploy start

# Check status
spdeploy status

# View real-time logs
spdeploy log -f
```

## 📋 Complete Command Reference

### Repository Management

#### `spdeploy repo add`
Add a repository to monitor for automatic deployment.

**Required Options:**
- `--repo <url>` - GitHub repository URL
- `--path <path>` - Local deployment path

**Optional Options:**
- `--branch <name>` - Branch to monitor (default: "main")
- `--trigger <type>` - When to deploy: `push`, `pr`, or `both` (default: "push")
- `--with-token` - Use stored GitHub token for this repository

**Examples:**
```bash
# Basic usage
spdeploy repo add --repo https://github.com/user/app --path ~/deployments/app

# Monitor production branch for PR merges
spdeploy repo add --repo https://github.com/user/api --branch production --path /var/www/api --trigger pr

# Monitor both pushes and PRs on develop branch
spdeploy repo add --repo https://github.com/user/webapp --branch develop --path ~/dev/webapp --trigger both
```

#### `spdeploy repo list`
List all monitored repositories.

```bash
spdeploy repo list
```

Output shows:
- Repository URL
- Branch being monitored
- Local deployment path
- Trigger type (push/pr/both)

#### `spdeploy repo remove`
Remove a repository from monitoring.

**Options:**
- `--repo <url>` - Repository URL (required)
- `--branch <name>` - Specific branch to remove (default: "main")
- `--path <path>` - Remove specific path only
- `--all` - Remove all instances of this repository

**Examples:**
```bash
# Remove specific repository/branch
spdeploy repo remove --repo https://github.com/user/app --branch main

# Remove all instances of a repository
spdeploy repo remove --repo https://github.com/user/app --all
```

#### `spdeploy repo auth`
Set up authentication for repositories.

**For SSH URLs:**
```bash
# Get SSH key setup instructions
spdeploy repo auth git@github.com:user/repo.git
```

**For HTTPS URLs:**
```bash
# Interactive Personal Access Token setup
spdeploy repo auth https://github.com/user/repo.git

# Clear stored token
spdeploy repo auth --logout
```

### Service Control

#### `spdeploy start`
Start the SPDeploy daemon service.

```bash
spdeploy start
```

#### `spdeploy stop`
Stop the SPDeploy daemon service.

```bash
spdeploy stop
```

#### `spdeploy status`
Show daemon status and monitored repositories count.

```bash
spdeploy status
```

### Installation Management

#### `spdeploy install`
Install SPDeploy for the current user.

```bash
spdeploy install
```

This command:
- Installs the binary to system location
- Creates user configuration directories
- Sets up platform-specific user service:
  - **Linux**: systemd user service
  - **macOS**: LaunchAgent
  - **Windows**: Scheduled Task

#### `spdeploy uninstall`
Uninstall SPDeploy from the system.

```bash
spdeploy uninstall
```

Interactive prompts for:
- Confirmation
- Whether to remove configuration and logs

### Log Management

#### `spdeploy log`
View deployment logs with various options.

**Options:**
- `-f, --follow` - Follow logs in real-time
- `-g, --global` - Show global service logs
- `--repo <url>` - Show logs for specific repository
- `-a, --all` - List all monitored repositories
- `--user <username>` - View logs for specific user (requires sudo)

**Examples:**
```bash
# View logs for current repository (auto-detected)
spdeploy log

# Follow logs in real-time
spdeploy log -f

# View global service logs
spdeploy log -g

# View logs for specific repository
spdeploy log --repo https://github.com/user/app

# List all repositories being monitored
spdeploy log -a
```

## ⚙️ Configuration

### Trigger Types

- **push**: Deploy when commits are pushed to the branch
- **pr**: Deploy when pull requests are merged to the branch
- **both**: Deploy on both pushes and PR merges

### Configuration File Locations

- **Linux**: `~/.config/spdeploy/config.yaml` (user) or `/etc/spdeploy/config.yaml` (root)
- **macOS**: `~/.config/spdeploy/config.yaml` (user) or `/etc/spdeploy/config.yaml` (root)
- **Windows**: `C:\ProgramData\SPDeploy\config.yaml`

### Log File Locations

- **Global logs**: `~/.spdeploy/logs/global/<username>/YYYY-MM-DD.log`
- **Repository logs**: `~/.spdeploy/logs/repos/<repo-name>/<username>/YYYY-MM-DD.log`

## 🔧 Deployment Scripts

SPDeploy automatically executes deployment scripts after pulling changes. Create one of these files in your repository root:

### Unix/Linux/macOS
Create `spdeploy.sh` in your repository root:

```bash
#!/bin/bash
# spdeploy.sh - Deployment script

echo "Starting deployment..."

# Install dependencies
npm install

# Build the application
npm run build

# Restart the service
pm2 restart app

echo "Deployment complete!"
```

### Windows
Create `spdeploy.bat`, `spdeploy.cmd`, or `spdeploy.ps1` in your repository root:

```batch
@echo off
REM spdeploy.bat - Deployment script

echo Starting deployment...

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

- `SPDEPLOY_REPO_PATH` - Path to the repository
- `SPDEPLOY_TIMESTAMP` - Deployment timestamp (RFC3339 format)
- `SPDEPLOY_VERSION` - SPDeploy version
- `SPDEPLOY_GIT_BRANCH` - Current Git branch
- `SPDEPLOY_GIT_COMMIT` - Current Git commit hash
- `SPDEPLOY_GIT_REMOTE` - Git remote URL

### Script Execution

- **Timeout**: 5 minutes (configurable)
- **Working Directory**: Repository root
- **Output**: Logged to repository-specific log files
- **Exit Codes**: Tracked and logged

## 🔒 Authentication

### SSH Keys (Recommended for Private Repos)

1. Generate an SSH key if you don't have one:
```bash
ssh-keygen -t ed25519 -C "your_email@example.com"
```

2. Add the public key to GitHub:
```bash
cat ~/.ssh/id_ed25519.pub
# Copy and add to GitHub Settings > SSH Keys
```

3. Use SSH URLs when adding repositories:
```bash
spdeploy repo add --repo git@github.com:user/repo.git --path ~/deploy/repo
```

### Personal Access Token (For HTTPS URLs)

1. Create a token on GitHub:
   - Go to Settings > Developer settings > Personal access tokens
   - Create a token with `repo` scope

2. Set up authentication:
```bash
# Interactive setup
spdeploy repo auth https://github.com/user/repo.git

# Or set environment variable
export GITHUB_TOKEN=your_token_here
```

## 🛠️ Building from Source

### Prerequisites
- Go 1.21 or later
- Git

### Build
```bash
# Clone the repository
git clone https://github.com/simonjcarr/spdeploy.git
cd spdeploy

# Build for all platforms (creates dist/ directory)
./build.sh all

# Build for current platform only
./build.sh local

# Create release packages
./build.sh all && ./build.sh package

# Run tests
./build.sh test
```

### Build Script Commands
- `./build.sh all` - Build for all platforms
- `./build.sh local` - Build for current platform
- `./build.sh package` - Create release packages (tar.gz/zip)
- `./build.sh test` - Run tests
- `./build.sh clean` - Clean build directory
- `./build.sh deps` - Install dependencies
- `./build.sh fmt` - Format code

## 📊 Example Workflows

### Local Development Environment
```bash
# Monitor team repository for development
spdeploy repo add --repo https://github.com/team/webapp --branch develop --path ~/dev/webapp --trigger both
spdeploy start

# View logs to monitor deployments
spdeploy log -f
```

### Staging Server
```bash
# Deploy on PR merges to main
spdeploy repo add --repo https://github.com/company/api --branch main --path /var/www/staging --trigger pr
spdeploy start

# Check deployment status
spdeploy status
spdeploy log --repo https://github.com/company/api -f
```

### Production Server
```bash
# Deploy only on pushes to production branch
spdeploy repo add --repo https://github.com/company/app --branch production --path /var/www/production --trigger push
spdeploy start

# Monitor deployments
spdeploy log -g -f  # Follow global logs
```

## 🐛 Troubleshooting

### Service Won't Start
```bash
# Check if already running
spdeploy status

# View logs for errors
spdeploy log -g

# Stop and restart
spdeploy stop
spdeploy start
```

### Repository Not Updating
```bash
# Check repository list and status
spdeploy repo list
spdeploy status

# View repository-specific logs
spdeploy log --repo https://github.com/user/repo -f

# Check authentication
spdeploy repo auth https://github.com/user/repo
```

### Permission Errors
```bash
# Ensure deployment path is writable
ls -la ~/deployment/path

# Check deployment script is executable
chmod +x spdeploy.sh

# View logs for detailed errors
spdeploy log -f
```

### Authentication Issues
```bash
# For SSH issues
ssh -T git@github.com  # Test SSH connection

# For HTTPS token issues
spdeploy repo auth --logout  # Clear token
spdeploy repo auth https://github.com/user/repo  # Re-authenticate
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