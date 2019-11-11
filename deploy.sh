#!/bin/bash
set -euo pipefail

BUILD_VERSION=$(date +"%Y%m%d_%H%M")
BUILD_ARCHIVE="localfm-${BUILD_VERSION}.tgz"

BINDIR="dist/${BUILD_VERSION}/bin"
STATICDIR="dist/${BUILD_VERSION}/static"

# clear previous build
rm -rf dist
mkdir -p $BINDIR
mkdir -p $STATICDIR

# build all binaries
export GOOS=linux
export GOARCH=amd64

echo "building web"
go build -o $BINDIR ./cmd/web/
echo "building update"
go build -o $BINDIR ./cmd/update/

echo "copying ui files"
cp -R ui $STATICDIR

pushd dist
tar czf ../${BUILD_ARCHIVE} ${BUILD_VERSION}
popd

echo "build ${BUILD_VERSION} successful"
ls -l ${BUILD_ARCHIVE}
