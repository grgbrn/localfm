#!/bin/bash
set -euo pipefail

BUILD_VERSION=$(date +"%Y%m%d_%H%M")
BUILD_ARCHIVE="localfm-${BUILD_VERSION}.tgz"

BINDIR="dist/"

# clear previous build
rm -rf dist
mkdir -p $BINDIR

# build all binaries
export GOOS=linux
export GOARCH=amd64

echo "building web"
go build -o $BINDIR ./cmd/web/
echo "building update"
go build -o $BINDIR ./cmd/update/

# docker images
docker build -t localfm-web .
docker build -t localfm-update -f Dockerfile.update .
