# Stage 1: Build the Go application
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy dependency files
COPY go.mod go.sum ./

# Download dependencies (since we did go mod tidy, they are listed here)
RUN go mod download

# Copy source code files
COPY src/ src/

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api-server ./src

# Stage 2: Final lightweight image
FROM alpine:latest

WORKDIR /app

# Install ca-certificates in case they are needed for SSL connections
RUN apk --no-cache add ca-certificates

# Copy the compiled binary from the builder stage
COPY --from=builder /app/api-server .

# Expose port 8080 (the default app port)
EXPOSE 8080

# Command to run the application
ENTRYPOINT ["./api-server"]
