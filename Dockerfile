# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/clipper-server/main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk --no-cache add ca-certificates

# Copy the binary from builder
COPY --from=builder /app/server .

# Create directories for config and media
RUN mkdir -p /app/config /app/media

# Set environment variables with defaults
ENV CLIPPER_CONTACTS_FILE=/app/config/contacts.json
ENV CLIPPER_MEDIA_DIR=/app/media
ENV CLIPPER_PORT=8080
ENV PID=99
ENV GUID=100

# Create a non-root user
RUN adduser -D -u ${PID} -g ${GUID} appuser

# Set ownership of directories
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose the port the app runs on
EXPOSE ${CLIPPER_PORT}

# Command to run the executable
CMD ["./server"] 