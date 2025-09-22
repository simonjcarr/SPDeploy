# SPDeploy - Universal Git Continuous Deployment with CI/CD Automation

SPDeploy is a free, open-source continuous deployment service that automatically syncs code from ANY git provider (GitHub, GitLab, BitBucket, etc.) to your servers and optionally executes custom deployment scripts for seamless CI/CD workflows. Built with simplicity in mind, SPDeploy bridges the gap between git push and production deployment.

**100% Open Source** • **MIT Licensed** • **No Vendor Lock-in** • **Self-Hosted** • **Single Binary** • **No complex Setup** • **Written in GO**

## 🚀 Quick Start (2 Minutes)

### Linux/macOS

```bash
# 1. Install
curl -sSL https://spdeploy.io/install.sh | sh

# 2. Set up authentication for your repository
# For HTTPS URLs - set token:
export SPDEPLOY_GITHUB_TOKEN=ghp_xxxx      # For GitHub
export SPDEPLOY_GITLAB_TOKEN=glpat_xxxx    # For GitLab
export SPDEPLOY_BITBUCKET_TOKEN=xxxx       # For BitBucket

# For SSH URLs - set up SSH keys:
spdeploy repo auth git@github.com:user/app.git  # Shows SSH setup instructions

# 3. Add repository to monitor
spdeploy repo add --repo https://github.com/user/app.git --branch main --path /var/www/app
# OR for SSH:
spdeploy repo add --repo git@github.com:user/app.git --branch main --path /var/www/app

# 4. Start auto-deployment
spdeploy start
```

### Windows

```powershell
# 1. Download from https://github.com/simonjcarr/spdeploy/releases
# 2. Extract spdeploy.exe to C:\Program Files\SPDeploy\

# 3. Set environment variable
$env:SPDEPLOY_GITHUB_TOKEN = "ghp_xxxx"

# 4. Add repository
spdeploy repo add --repo https://github.com/user/app.git --branch main --path C:\inetpub\app

# 5. Install and start service
spdeploy install
spdeploy start
```

## 📦 Installation

### Linux

```bash
# AMD64
curl -L https://github.com/simonjcarr/spdeploy/releases/latest/download/spdeploy-linux-amd64.tar.gz | tar -xz
sudo mv spdeploy /usr/local/bin/
# Install the service (do NOT use sudo)
spdeploy install

# ARM64 (Raspberry Pi, AWS Graviton)
curl -L https://github.com/simonjcarr/spdeploy/releases/latest/download/spdeploy-linux-arm64.tar.gz | tar -xz
sudo mv spdeploy /usr/local/bin/
# Install the service (do NOT use sudo)
spdeploy install
```

### macOS

```bash
# Intel Macs
curl -L https://github.com/simonjcarr/spdeploy/releases/latest/download/spdeploy-darwin-amd64.tar.gz | tar -xz
sudo mv spdeploy /usr/local/bin/
# Install the service (do NOT use sudo)
spdeploy install

# Apple Silicon (M1/M2/M3)
curl -L https://github.com/simonjcarr/spdeploy/releases/latest/download/spdeploy-darwin-arm64.tar.gz | tar -xz
sudo mv spdeploy /usr/local/bin/
# Install the service (do NOT use sudo)
spdeploy install
```

### Windows

Download from [releases page](https://github.com/simonjcarr/spdeploy/releases) and extract to `C:\Program Files\SPDeploy\`, then add to PATH.

## ⚠️ Important: Sudo Usage

**DO NOT use sudo with `spdeploy install` or `spdeploy start`!**

- Run `spdeploy install` as a normal user - it will prompt for sudo when needed
- Run `spdeploy start` as a normal user - the service runs under your user account
- Running with sudo causes the service to be configured for root instead of your user

```bash
# CORRECT ✅
spdeploy install  # Prompts for sudo when needed
spdeploy start    # Runs as your user

# WRONG ❌
sudo spdeploy install  # Configures for root user
sudo spdeploy start    # Runs as root
```

## 🔑 Authentication Setup

### Environment Variables (Recommended)

SPDeploy uses environment variables for secure token storage. Set the appropriate variable for your provider:

```bash
# GitHub
export SPDEPLOY_GITHUB_TOKEN=ghp_xxxxxxxxxxxx

# GitLab
export SPDEPLOY_GITLAB_TOKEN=glpat_xxxxxxxxxxxx

# BitBucket
export SPDEPLOY_BITBUCKET_TOKEN=xxxxxxxxxxxx

# Self-hosted (after registering provider)
export SPDEPLOY_GITLAB_COMPANY_TOKEN=glpat_xxxxxxxxxxxx
```

### Getting Tokens

**GitHub:**
1. Go to https://github.com/settings/tokens/new
2. Select 'repo' scope
3. Generate token

**GitLab:**
1. Go to https://gitlab.com/-/profile/personal_access_tokens
2. Select 'read_repository' scope
3. Create token

**BitBucket:**
1. Go to https://bitbucket.org/account/settings/app-passwords/
2. Create app password with repository read access

## 🌐 Provider Support

### Built-in Providers

SPDeploy automatically detects and supports:
- GitHub (github.com)
- GitLab (gitlab.com)
- BitBucket (bitbucket.org)

### Self-Hosted Git Servers

```bash
# Register self-hosted GitLab
spdeploy provider add gitlab-company gitlab https://gitlab.company.com
export SPDEPLOY_GITLAB_COMPANY_TOKEN=glpat_xxxx

# Register GitHub Enterprise
spdeploy provider add github-corp github https://github.company.com
export SPDEPLOY_GITHUB_CORP_TOKEN=ghp_xxxx

# Add repository from self-hosted provider
spdeploy repo add --repo https://gitlab.company.com/team/api.git --branch main --path /var/www/api
```

### Auto-Detection

SPDeploy automatically detects the git provider:

```bash
# Test detection
spdeploy provider detect https://git.company.com/project.git
```

## 📋 Commands

### Core Commands

```bash
# Add repository to monitor
spdeploy repo add --repo <repo-url> --branch <branch> --path <deploy-path>

# Start/stop service
spdeploy start
spdeploy stop
spdeploy status

# View logs
spdeploy log               # All logs
spdeploy log -f            # Follow mode
spdeploy log --repo <url>  # Specific repo
```

### Provider Management

```bash
# List all providers
spdeploy provider list

# Add self-hosted provider
spdeploy provider add <name> <type> <url>

# Test provider connectivity
spdeploy provider test <name>

# Remove provider
spdeploy provider remove <name>
```

### Repository Management

```bash
# List monitored repositories
spdeploy repo list

# Remove repository
spdeploy repo remove --repo <repo-url> --branch <branch>

# Sync repository manually
spdeploy repo sync --id <repo-id>
```

## 💻 Examples by Use Case

### Development Environment

```bash
# Monitor multiple repos from different providers
export SPDEPLOY_GITHUB_TOKEN=ghp_xxxx
export SPDEPLOY_GITLAB_TOKEN=glpat_xxxx

spdeploy repo add --repo https://github.com/team/frontend.git --branch develop --path ~/dev/frontend
spdeploy repo add --repo https://gitlab.com/team/backend.git --branch develop --path ~/dev/backend
spdeploy start
```

### Production Server

```bash
# Deploy only on main branch updates
spdeploy repo add --repo https://github.com/company/app.git --branch main --path /var/www/production

# Add deployment script to your repo (spdeploy.sh)
cat > spdeploy.sh << 'EOF'
#!/bin/bash
npm install --production
npm run build
pm2 restart app
EOF

spdeploy start
```

### Multi-Provider Setup

```bash
# GitHub public + GitLab private + BitBucket
export SPDEPLOY_GITLAB_TOKEN=glpat_xxxx
export SPDEPLOY_BITBUCKET_TOKEN=xxxx

spdeploy repo add --repo https://github.com/open/project.git --branch main --path ~/public
spdeploy repo add --repo https://gitlab.com/private/api.git --branch main --path ~/api
spdeploy repo add --repo https://bitbucket.org/team/lib.git --branch main --path ~/lib
spdeploy start
```

## 🔧 Deployment Scripts

SPDeploy runs `spdeploy.sh` (Linux/macOS) or `spdeploy.bat` (Windows) after pulling changes:

```bash
#!/bin/bash
# spdeploy.sh - Runs after each pull

# Install dependencies
npm ci --production

# Build
npm run build

# Restart service
systemctl --user restart myapp

# Notify completion
curl -X POST https://hooks.slack.com/services/xxx -d '{"text":"Deployed!"}'
```

## 🐳 Docker Support

```yaml
# docker-compose.yml
version: '3.8'
services:
  spdeploy:
    image: spdeploy/spdeploy:latest
    environment:
      - SPDEPLOY_GITHUB_TOKEN=${GITHUB_TOKEN}
      - SPDEPLOY_GITLAB_TOKEN=${GITLAB_TOKEN}
    volumes:
      - ./config.yaml:/etc/spdeploy/config.yaml
      - /var/www:/var/www
```

## ⚙️ Configuration

### Config File Location

- **Linux/macOS**: `~/.config/spdeploy/config.yaml` or `/etc/spdeploy/config.yaml`
- **Windows**: `C:\ProgramData\SPDeploy\config.yaml`

### Example Config

```yaml
repositories:
  - id: webapp
    url: https://github.com/team/webapp.git
    branch: main
    path: /var/www/webapp
    provider: github      # Auto-detected if omitted
    auth_method: pat      # pat or ssh

  - id: api
    url: https://gitlab.company.com/team/api.git
    branch: production
    path: /var/www/api
    provider: gitlab

providers:
  - name: gitlab-company
    type: gitlab
    base_url: https://gitlab.company.com
    api_url: https://gitlab.company.com/api/v4

poll_interval: 60
log_level: info
```

## 🔒 Security

- **Never commit tokens** - Use environment variables only
- **Minimal token scopes** - Only grant necessary permissions
- **Rotate tokens regularly** - Update tokens every 90 days
- **Use SSH for production** - More secure than token auth
- **Monitor logs** - Check `spdeploy logs` for unauthorized access

## 🛠️ Build from Source

```bash
git clone https://github.com/simonjcarr/spdeploy.git
cd spdeploy
go build -o spdeploy cmd/spdeploy/*.go
```

## 📊 Performance

- **Lightweight**: < 15MB binary, < 20MB RAM usage
- **Fast**: Polls every 60 seconds (configurable)
- **Efficient**: Only pulls when changes detected
- **Scalable**: Monitor unlimited repositories

## 🐛 Troubleshooting

### Service won't start
```bash
spdeploy status          # Check if already running
spdeploy logs -f         # View error logs
spdeploy start           # Start as normal user (do NOT use sudo)
```

### Authentication failed
```bash
# Test token
spdeploy provider test github

# Re-export token
export SPDEPLOY_GITHUB_TOKEN=ghp_new_token

# Verify environment
env | grep SPDEPLOY
```

### Repository not updating
```bash
# Check repo status
spdeploy repo list

# Manual sync
spdeploy repo sync --id <repo-id>

# Check logs
spdeploy log --repo <repo-url> -f
```

## 🤝 Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## 📝 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

Built with:
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [go-git](https://github.com/go-git/go-git) - Git operations
- [Zap](https://go.uber.org/zap) - Logging

## 📚 More Resources

- [Provider Setup Guide](PROVIDER_GUIDE.md) - Detailed provider configuration
- [API Documentation](https://docs.spdeploy.io) - REST API reference
- [Examples](examples/) - Sample configurations and scripts
- [Discussions](https://github.com/simonjcarr/spdeploy/discussions) - Community support