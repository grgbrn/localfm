# syntax=docker/dockerfile:1
FROM debian:stable-slim
WORKDIR /app
# XXX remove this!
COPY localfm.db /app
COPY dist/web /app
COPY ui /app/ui
EXPOSE 4000
CMD ["/app/web"]
