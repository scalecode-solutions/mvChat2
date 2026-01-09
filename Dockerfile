# mvChat2 Docker image
# Build: docker build -t mvchat2:latest .
# Run: docker run -p 6060:6060 -e DB_HOST=host.docker.internal mvchat2:latest

FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X main.buildstamp=$(date -u '+%Y%m%dT%H%M%SZ')" -o mvchat2 .

# Runtime image
FROM alpine:3.21

LABEL maintainer="ScaleCode Solutions"
LABEL name="mvChat2"

RUN apk add --no-cache \
    ca-certificates \
    ffmpeg \
    imagemagick

WORKDIR /app

COPY --from=builder /build/mvchat2 .
COPY --from=builder /build/mvchat2.yaml .

RUN mkdir -p uploads static

ENV DB_HOST=localhost
ENV DB_PORT=5432
ENV DB_NAME=mvchat2
ENV DB_USER=postgres
ENV DB_PASSWORD=
# Security keys - MUST be provided at runtime via docker-compose or -e flags
# Generate with: ./mvchat2 --generate-keys
ENV UID_KEY=
ENV ENCRYPTION_KEY=
ENV API_KEY_SALT=
ENV TOKEN_KEY=

EXPOSE 6060

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
    CMD wget -q --spider http://localhost:6060/health || exit 1

ENTRYPOINT ["./mvchat2"]
