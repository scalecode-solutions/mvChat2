# mvChat2 Docker image
# Build: docker build -t mvchat2:latest .
# Run: docker run -p 6060:6060 -e DB_HOST=host.docker.internal mvchat2:latest

FROM golang:1.24-alpine AS builder

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
ENV UID_KEY=la6YsO+bNX/+XIkOqc5Svw==
ENV ENCRYPTION_KEY=
ENV API_KEY_SALT=T713/rYYgW7g4m3vG6zGRh7+FM1t0T8j13koXScOAj4=
ENV TOKEN_KEY=wfaY2RgF2S1OQI/ZlK+LSrp1KB2jwAdGAIHQ7JZn+Kc=

EXPOSE 6060

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
    CMD wget -q --spider http://localhost:6060/health || exit 1

ENTRYPOINT ["./mvchat2"]
