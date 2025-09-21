#!/bin/bash

# GoCD Build Script
# Cross-compiles for multiple platforms

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

APP_NAME="gocd"
VERSION=${VERSION:-"1.0.0"}
DIST_DIR="dist"

# Helper functions
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Clean dist directory
clean() {
    log_info "Cleaning dist directory..."
    rm -rf ${DIST_DIR}
    mkdir -p ${DIST_DIR}
}

# Build for single platform
build_platform() {
    local os=$1
    local arch=$2
    local ext=$3

    local platform_dir="${DIST_DIR}/${os}-${arch}"
    mkdir -p ${platform_dir}

    log_info "Building for ${os}/${arch}..."

    # Build single binary
    CGO_ENABLED=0 GOOS=${os} GOARCH=${arch} go build \
        -ldflags "-X main.Version=${VERSION} -s -w" \
        -trimpath \
        -o ${platform_dir}/${APP_NAME}${ext} \
        ./cmd/gocd

    log_success "Built ${os}/${arch} → ${platform_dir}"
}

# Build for current platform only
build_local() {
    log_info "Building for current platform..."

    # Detect current platform
    local os=$(go env GOOS)
    local arch=$(go env GOARCH)
    local ext=""

    if [ "$os" = "windows" ]; then
        ext=".exe"
    fi

    # Build single binary
    go build -ldflags "-X main.Version=${VERSION} -s -w" -trimpath -o ${APP_NAME}${ext} ./cmd/gocd

    log_success "Built ${APP_NAME} for ${os}/${arch}"
}

# Build for all supported platforms
build_all() {
    log_info "Building for all supported platforms..."

    # Linux
    build_platform "linux" "amd64" ""
    build_platform "linux" "arm64" ""

    # macOS
    build_platform "darwin" "amd64" ""
    build_platform "darwin" "arm64" ""

    # Windows
    build_platform "windows" "amd64" ".exe"

    echo ""
    log_success "All platforms built successfully!"
    log_info "Binaries available in:"
    ls -la ${DIST_DIR}/
}

# Create release packages
package() {
    log_info "Creating release packages..."

    mkdir -p ${DIST_DIR}/releases

    for platform_dir in ${DIST_DIR}/*/; do
        if [ -d "$platform_dir" ]; then
            platform=$(basename "$platform_dir")

            # Skip releases directory
            if [ "$platform" = "releases" ]; then
                continue
            fi

            if [[ "$platform" == *"windows"* ]]; then
                # Create ZIP for Windows
                (cd "$platform_dir" && zip -r "../releases/${APP_NAME}-${VERSION}-${platform}.zip" .)
                log_success "Created ZIP for $platform"
            else
                # Create tar.gz for Unix-like systems
                tar -czf "${DIST_DIR}/releases/${APP_NAME}-${VERSION}-${platform}.tar.gz" -C "$platform_dir" .
                log_success "Created tar.gz for $platform"
            fi
        fi
    done

    log_success "All packages created in ${DIST_DIR}/releases/"
}

# Install dependencies
deps() {
    log_info "Installing dependencies..."
    go mod download
    go mod verify
    log_success "Dependencies installed"
}

# Run tests
test() {
    log_info "Running tests..."
    go test -v ./...
    log_success "Tests completed"
}

# Format code
fmt() {
    log_info "Formatting code..."
    go fmt ./...
    log_success "Code formatted"
}

# Show help
show_help() {
    echo "GoCD Build Script"
    echo ""
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  all       Build for all supported platforms (default)"
    echo "  local     Build for current platform only"
    echo "  package   Create release packages (requires 'all' first)"
    echo "  clean     Clean dist directory"
    echo "  deps      Install dependencies"
    echo "  test      Run tests"
    echo "  fmt       Format code"
    echo "  help      Show this help"
    echo ""
    echo "Environment variables:"
    echo "  VERSION   Set version string (default: 1.0.0)"
    echo ""
    echo "Examples:"
    echo "  $0              # Build for all platforms"
    echo "  $0 all          # Build for all platforms"
    echo "  $0 local        # Build for current platform only"
    echo "  VERSION=1.2.3 $0 all"
    echo "  $0 all && $0 package"
}

# Main execution
main() {
    case "${1:-all}" in
        "clean")
            clean
            ;;
        "deps")
            deps
            ;;
        "local")
            clean
            deps
            build_local
            ;;
        "all"|"")
            clean
            deps
            build_all
            ;;
        "package")
            package
            ;;
        "test")
            test
            ;;
        "fmt")
            fmt
            ;;
        "help"|"-h"|"--help")
            show_help
            ;;
        *)
            log_error "Unknown command: $1"
            echo ""
            show_help
            exit 1
            ;;
    esac
}

# Check if Go is installed
if ! command -v go &> /dev/null; then
    log_error "Go is not installed or not in PATH"
    exit 1
fi

# Check if we're in the right directory
if [ ! -f "go.mod" ]; then
    log_error "go.mod not found. Are you in the project root?"
    exit 1
fi

# Clean up old build directory if it exists
if [ -d "build" ]; then
    log_info "Removing old build directory..."
    rm -rf build
fi

# Run main function
main "$@"