FROM golang:1.24-alpine AS builder

# Install git for go mod download
RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o mike-ai .

FROM alpine:latest

# Add ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Create non-root user for security
RUN addgroup -g 1000 appgroup && \
    adduser -D -s /bin/sh -u 1000 -G appgroup appuser

COPY --from=builder /app/mike-ai .
COPY --from=builder /app/assets /var/www/mike-assets
COPY --from=builder /app/schema.json .
COPY --from=builder /app/instructions.txt .
RUN chown -R appuser:appgroup /app /var/www/mike-assets

USER appuser
EXPOSE 8080
CMD ["./mike-ai"]
