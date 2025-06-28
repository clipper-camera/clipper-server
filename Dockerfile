ARG BUILD_FROM=alpine:latest

# Build arguments for CLI flexibility
ARG GO_VERSION=1.22
ARG CONTACTS_FILE=/config/clipper/contacts.json
ARG PORT=8080

FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy only necessary source files for building
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/clipper-server/main.go

# Final stage
FROM ${BUILD_FROM}

# Install runtime dependencies
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/server .

# Create data directory structure
RUN mkdir -p /data/clipper/media

# Copy run script and LICENSE
COPY run.sh LICENSE ./
RUN chmod a+x ./run.sh

# Set environment variables with defaults from build args
ENV CLIPPER_CONTACTS_FILE=${CONTACTS_FILE}
ENV CLIPPER_MEDIA_DIR=/data/clipper/media
ENV CLIPPER_PORT=${PORT}

# Expose the port the app runs on
EXPOSE ${CLIPPER_PORT}

# Command to run the executable
CMD [ "./run.sh" ] 