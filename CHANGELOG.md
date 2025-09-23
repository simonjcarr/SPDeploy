# Changelog

All notable changes to SPDeploy will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v3.0.2] - 2025-09-23

### Added
- CHANGELOG.md file for tracking version history and release notes
- Automated changelog extraction in GitHub release workflow

### Fixed
- GitHub releases now properly display changelog content for each version

## [v3.0.1] - 2024-09-23

### Changed
- Complete rewrite of README.md for better user experience
- Added clear value proposition and quick start guide
- Reorganized documentation with improved structure
- Added comprehensive usage examples and troubleshooting section
- Improved clarity for new users
- Emphasized simplicity and zero-configuration approach

## [v3.0.0] - 2024-09-23

### Added
- Comprehensive test coverage for all components
- Support for multiple Git providers (GitHub, GitLab, BitBucket)
- Universal repository monitoring from any Git server
- SSH-based authentication system
- Background daemon support
- Real-time log following with `log -f` command
- Post-pull script execution support
- Branch selection support

### Changed
- Complete rewrite using basic git commands for better reliability
- Simplified architecture removing unnecessary dependencies
- Consolidated logger implementations
- Improved daemon management (only one instance allowed)
- Enhanced logging system for better debugging

### Removed
- Unused code and redundant implementations
- Complex provider-specific authentication methods
- Dependency on external Git libraries

### Fixed
- Log following functionality (`log -f` command)
- Repository synchronization issues
- Daemon process management

## [v2.1.0] - 2024-09-22

### Added
- Enhanced build metadata in CI/CD pipeline
- Improved version management system

### Fixed
- Uninstall process now works correctly
- Repository loading from GitLab and GitHub

## [v2.0.0] - 2024-09-22

### Changed
- Project renamed from GoCD to SPDeploy
- Complete refactoring of codebase structure
- Enhanced build workflow with version management

### Added
- Claude Code Review workflow for pull requests
- GitHub Actions integration

### Fixed
- Printf formatting directive issues
- Build errors in main.go

## [v1.0.1] - 2024-09-21

### Changed
- Updated README with new CLI command structure
- Improved documentation

## [v1.0.0] - 2024-09-21

### Added
- Initial release of SPDeploy (formerly GoCD)
- Basic repository monitoring functionality
- CLI interface with commands:
  - `add` - Add repositories to monitor
  - `list` - List monitored repositories
  - `remove` - Remove repositories
  - `run` - Start monitoring
  - `stop` - Stop monitoring daemon
  - `log` - View deployment logs
- Build and release workflows for CI/CD
- Support for Linux, macOS, and Windows
- ARM and AMD64 architecture support
- Automatic Git pull on repository changes
- SSH key authentication
- Configuration file support

[Unreleased]: https://github.com/simonjcarr/spdeploy/compare/v3.0.2...HEAD
[v3.0.2]: https://github.com/simonjcarr/spdeploy/compare/v3.0.1...v3.0.2
[v3.0.1]: https://github.com/simonjcarr/spdeploy/compare/v3.0.0...v3.0.1
[v3.0.0]: https://github.com/simonjcarr/spdeploy/compare/v2.1.0...v3.0.0
[v2.1.0]: https://github.com/simonjcarr/spdeploy/compare/v2.0.0...v2.1.0
[v2.0.0]: https://github.com/simonjcarr/spdeploy/compare/v1.0.1...v2.0.0
[v1.0.1]: https://github.com/simonjcarr/spdeploy/compare/v1.0.0...v1.0.1
[v1.0.0]: https://github.com/simonjcarr/spdeploy/releases/tag/v1.0.0