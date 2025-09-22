# SPDeploy Git Provider Guide

SPDeploy now supports any git provider (GitHub, GitLab, BitBucket, etc.) through a flexible provider system. This guide shows how to configure and use different git providers.

## Quick Start Examples

### GitHub.com Repository

```bash
# Set your GitHub token
export SPDEPLOY_GITHUB_TOKEN=ghp_xxxxxxxxxxxx

# Add a GitHub repository
spdeploy add https://github.com/user/repo.git main /var/www/app

# Start the service
sudo spdeploy start
```

### GitLab.com Repository

```bash
# Set your GitLab token
export SPDEPLOY_GITLAB_TOKEN=glpat_xxxxxxxxxxxx

# Add a GitLab repository
spdeploy add https://gitlab.com/user/repo.git main /var/www/app

# Start the service
sudo spdeploy start
```

### Self-Hosted GitLab

```bash
# Register your GitLab instance
spdeploy provider add gitlab-company gitlab https://gitlab.company.com

# Set the token for this instance
export SPDEPLOY_GITLAB_COMPANY_TOKEN=glpat_xxxxxxxxxxxx

# Add a repository from your GitLab
spdeploy add https://gitlab.company.com/team/api.git main /var/www/api

# Start the service
sudo spdeploy start
```

### GitHub Enterprise

```bash
# Register your GitHub Enterprise instance
spdeploy provider add github-corp github https://github.company.com

# Set the token
export SPDEPLOY_GITHUB_CORP_TOKEN=ghp_xxxxxxxxxxxx

# Add a repository
spdeploy add https://github.company.com/team/app.git main /var/www/app

# Start the service
sudo spdeploy start
```

### BitBucket Server

```bash
# Register BitBucket Server
spdeploy provider add bitbucket-internal bitbucket https://bitbucket.internal.net

# Set the token
export SPDEPLOY_BITBUCKET_INTERNAL_TOKEN=xxxxxxxxxxxx

# Add a repository
spdeploy add https://bitbucket.internal.net/project/repo.git main /var/www/app

# Start the service
sudo spdeploy start
```

## Provider Management Commands

### Detect Provider Type

Automatically detect what type of git server a URL points to:

```bash
spdeploy provider detect https://git.company.com/user/repo.git
```

### List Providers

Show all configured providers:

```bash
spdeploy provider list
```

### Add Provider Instance

Register a self-hosted git provider:

```bash
# Basic registration
spdeploy provider add <name> <type> <base-url>

# With custom API URL
spdeploy provider add gitlab-dev gitlab https://gitlab.dev.company.com \
  --api-url https://gitlab.dev.company.com/api/v4
```

### Test Provider

Test connectivity and authentication:

```bash
# Test with environment token
spdeploy provider test github

# Test with specific token
spdeploy provider test gitlab --token glpat_xxxxxxxxxxxx
```

### Remove Provider

Remove a configured provider instance:

```bash
spdeploy provider remove gitlab-company
```

## Environment Variables

### Token Resolution Priority

SPDeploy looks for tokens in this order:

1. **Repository-specific**: `SPDEPLOY_REPO_<ID>_TOKEN`
2. **Instance-specific**: `SPDEPLOY_<INSTANCE_NAME>_TOKEN`
3. **Provider default**: `SPDEPLOY_<PROVIDER>_TOKEN`

### Examples

```bash
# Default GitHub token (for github.com)
export SPDEPLOY_GITHUB_TOKEN=ghp_xxxxxxxxxxxx

# Default GitLab token (for gitlab.com)
export SPDEPLOY_GITLAB_TOKEN=glpat_xxxxxxxxxxxx

# Self-hosted GitLab instance
export SPDEPLOY_GITLAB_COMPANY_TOKEN=glpat_xxxxxxxxxxxx

# Specific repository (highest priority)
export SPDEPLOY_REPO_APP_TOKEN=ghp_xxxxxxxxxxxx
```

## Configuration File

The config file now supports provider information:

```yaml
# ~/.config/spdeploy/config.yaml or /etc/spdeploy/config.yaml

repositories:
  - id: app
    url: https://github.com/user/app.git
    branch: main
    path: /var/www/app
    provider: github        # Auto-detected if not specified
    auth_method: pat        # pat, ssh, or oauth

  - id: api
    url: https://gitlab.company.com/team/api.git
    branch: main
    path: /var/www/api
    provider: gitlab
    auth_method: pat

providers:
  - name: gitlab-company
    type: gitlab
    base_url: https://gitlab.company.com
    api_url: https://gitlab.company.com/api/v4

  - name: github-enterprise
    type: github
    base_url: https://github.company.com
    api_url: https://github.company.com/api/v3
```

## Authentication Methods

### Personal Access Tokens (Recommended)

Most secure and widely supported:

```bash
# GitHub
export SPDEPLOY_GITHUB_TOKEN=ghp_xxxxxxxxxxxx

# GitLab
export SPDEPLOY_GITLAB_TOKEN=glpat_xxxxxxxxxxxx

# BitBucket
export SPDEPLOY_BITBUCKET_TOKEN=xxxxxxxxxxxx
```

### SSH Keys

For SSH URLs (no token needed):

```bash
# Add repository with SSH URL
spdeploy add git@github.com:user/repo.git main /var/www/app

# Ensure SSH keys are available
ssh-add ~/.ssh/id_ed25519
```

### OAuth (Future)

OAuth flow support is planned for interactive authentication.

## Docker/Kubernetes Integration

### Docker Compose

```yaml
version: '3.8'
services:
  spdeploy:
    image: spdeploy:latest
    environment:
      - SPDEPLOY_GITHUB_TOKEN
      - SPDEPLOY_GITLAB_TOKEN
      - SPDEPLOY_GITLAB_COMPANY_TOKEN
    volumes:
      - ./config.yaml:/etc/spdeploy/config.yaml
      - /var/www:/var/www
```

### Kubernetes

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: spdeploy-tokens
type: Opaque
stringData:
  SPDEPLOY_GITHUB_TOKEN: ghp_xxxxxxxxxxxx
  SPDEPLOY_GITLAB_TOKEN: glpat_xxxxxxxxxxxx
  SPDEPLOY_GITLAB_COMPANY_TOKEN: glpat_xxxxxxxxxxxx
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: spdeploy
spec:
  template:
    spec:
      containers:
      - name: spdeploy
        image: spdeploy:latest
        envFrom:
        - secretRef:
            name: spdeploy-tokens
        volumeMounts:
        - name: config
          mountPath: /etc/spdeploy
        - name: deployments
          mountPath: /var/www
```

## CI/CD Integration

### GitHub Actions

```yaml
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Deploy with SPDeploy
        env:
          SPDEPLOY_GITHUB_TOKEN: ${{ secrets.DEPLOY_TOKEN }}
          SPDEPLOY_GITLAB_TOKEN: ${{ secrets.GITLAB_TOKEN }}
        run: |
          spdeploy add ${{ github.repository }} main /var/www/app
          spdeploy sync
```

### GitLab CI

```yaml
deploy:
  stage: deploy
  script:
    - export SPDEPLOY_GITLAB_TOKEN=$CI_JOB_TOKEN
    - spdeploy add $CI_PROJECT_URL main /var/www/app
    - spdeploy sync
```

## Troubleshooting

### Test Provider Detection

```bash
# Check what provider SPDeploy detects
spdeploy provider detect https://git.example.com/repo.git
```

### Verify Token

```bash
# Check if token is set
echo $SPDEPLOY_GITHUB_TOKEN

# Test token validity
spdeploy provider test github
```

### Debug Mode

```bash
# Run with debug logging
SPDEPLOY_LOG_LEVEL=debug spdeploy start
```

### Common Issues

1. **"No token found"**: Check environment variable is set and exported
2. **"Invalid token"**: Verify token has correct permissions (repo access)
3. **"Unable to detect provider"**: Manually register with `spdeploy provider add`
4. **"Authentication failed"**: Check token permissions and expiration

## Provider-Specific Notes

### GitHub
- Requires `repo` scope for private repositories
- Token format: `ghp_xxxxxxxxxxxx`
- API rate limits apply

### GitLab
- Requires `read_repository` scope minimum
- Token format: `glpat_xxxxxxxxxxxx`
- Self-hosted may have different API paths

### BitBucket
- Use app passwords for Bitbucket Cloud
- Different API for Server vs Cloud

### Generic Git
- Falls back to basic auth
- SSH recommended for unknown providers

## Security Best Practices

1. **Never commit tokens** to repositories
2. **Use environment variables** or secret managers
3. **Rotate tokens regularly**
4. **Use minimal token scopes**
5. **Different tokens per environment**
6. **Monitor token usage** in provider dashboards

## Migration from Old Version

If upgrading from the GitHub-only version:

1. Existing GitHub repositories continue to work
2. Set `SPDEPLOY_GITHUB_TOKEN` from your old token
3. New repositories can use any provider

The system is backward compatible - old configurations will continue to work.