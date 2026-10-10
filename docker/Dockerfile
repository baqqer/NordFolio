# Stage 1: Build a static Go binary with the frontend embedded
FROM golang:alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
COPY internal/ ./internal/
COPY static/ ./static/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /nordfolio .

# Stage 2: Minimal runtime image. NordFolio makes no outbound requests,
# so no CA certificates are needed.
FROM alpine:latest

WORKDIR /app
COPY --from=builder /nordfolio ./nordfolio

EXPOSE 8080

ENTRYPOINT ["./nordfolio"]
