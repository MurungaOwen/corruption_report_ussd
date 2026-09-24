# syntax=docker/dockerfile:1
#
# Multi-stage build producing a static, CGO-free binary (the sqlite driver
# is pure Go — modernc.org/sqlite), so the final image can be `scratch`
# plus a CA bundle for any future outbound HTTPS (e.g. a real SMS gateway).
# No libc, no shared libraries, nothing to patch for a CVE that isn't ours.

FROM golang:1-alpine AS build
WORKDIR /src

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server
RUN go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

FROM scratch AS final
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/server /server
COPY --from=build /out/seed /seed
COPY web /web

ENV ADDR=:8080 \
    WEB_DIR=/web \
    DB_PATH=/data/reports.db \
    UPLOADS_DIR=/data/uploads \
    ENVIRONMENT=production

EXPOSE 8080
VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/server", "healthcheck"]

ENTRYPOINT ["/server"]
