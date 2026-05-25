# Use local golang:1.20 image for building
FROM golang:1.20 AS builder

ARG GOARCH=amd64

WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${GOARCH} go build -a -o device-plugin ./cmd/k8s-device-plugin

# Use alpine as base image
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /workspace/device-plugin .
ENTRYPOINT ["./device-plugin"]