#!/bin/bash
# Build and push release images to DockerHub
# Usage: ./release.sh [-v VERSION] [--dry-run]

set -e

VERSION=""
DRY_RUN=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: ./release.sh [-v VERSION] [--dry-run]"
            exit 1
            ;;
    esac
done

VERSION_ARG=""
if [ -n "$VERSION" ]; then
    VERSION_ARG="VERSION=$VERSION"
fi

echo -e "\033[36m=== Building release images ===\033[0m"
make docker-build-release $VERSION_ARG

if [ "$DRY_RUN" = true ]; then
    echo ""
    echo -e "\033[33m=== Dry run - skipping push ===\033[0m"
    exit 0
fi

echo ""
echo -e "\033[36m=== Pushing version tag ===\033[0m"
make docker-push-version $VERSION_ARG

echo ""
echo -e "\033[36m=== Pushing latest tag ===\033[0m"
make docker-push-latest

echo ""
echo -e "\033[32m=== Release complete ===\033[0m"

