# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.Version=${VERSION}" -o /out/vecto ./cmd/vecto

# Minimal runtime image
FROM alpine:3.21

RUN apk add --no-cache git bash ca-certificates coreutils

COPY --from=builder /out/vecto /usr/local/bin/vecto

WORKDIR /workspace

ENTRYPOINT ["vecto"]
CMD ["--help"]
