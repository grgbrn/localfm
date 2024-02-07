#
# Dockerfile based off:
# https://raw.githubusercontent.com/GoogleCloudPlatform/golang-samples/main/run/helloworld/Dockerfile
#
# use a builder to worry less about static linking, the debian
# based images should have similar-enough dynamic libs
#
# Use the offical golang image to create a binary.
# This is based on Debian and sets the GOPATH to /go.
# https://hub.docker.com/_/golang
FROM golang:1.21.7-bookworm as builder

WORKDIR /app

COPY go.* ./
RUN go mod download

COPY . ./
RUN go build -o localfm-web ./cmd/web

FROM debian:bookworm-slim
RUN set -x && apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y \
    ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/localfm-web /app
COPY --from=builder /app/ui/html /app/ui/html
COPY --from=builder /app/ui/static /app/ui/static

# Run the web service on container startup.
CMD ["/app/localfm-web"]
