FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
RUN CGO_ENABLED=0 go build -o capture-controller ./cmd/controller

FROM ubuntu:24.04
RUN apt-get update && apt-get install -y bash tcpdump && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/capture-controller /usr/local/bin/
ENTRYPOINT ["capture-controller"]
