# Stage 1: Build the Go static binary
FROM golang:alpine AS builder

WORKDIR /src

# Copy module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the application source code
COPY main.go ./
COPY internal/ ./internal/

# Build as a highly-optimized, statically-linked linux binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /nordnet-analyzer .

# Stage 2: Create a minimal production container
FROM alpine:latest

# Install CA certificates to enable outbound HTTPS API proxy fetches
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary and static assets from builder stage
COPY --from=builder /nordnet-analyzer ./nordnet-analyzer
COPY static/ ./static/

# Expose default port
EXPOSE 8080

# Run the app
ENTRYPOINT ["./nordnet-analyzer"]
