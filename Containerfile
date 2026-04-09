# syntax=docker/dockerfile:1

FROM golang:1.23-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o synesis ./cmd/synesis

FROM alpine:3.19

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /build/synesis /usr/local/bin/

ENTRYPOINT ["synesis"]