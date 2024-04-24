FROM golang:alpine3.19 AS builder
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download all dependencies
# RUN go mod download

# Vendor dependencies
RUN go mod vendor

# Copy the entire project
COPY . .

# Build the Go app
RUN GOOS=linux go build -mod vendor -o /k8s-pod-restart-info-collector .

# Start a new stage from scratch
FROM alpine:3.19.1
COPY --from=builder /k8s-pod-restart-info-collector /k8s-pod-restart-info-collector
CMD ["/k8s-pod-restart-info-collector"]
