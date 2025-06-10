# Build stage
FROM golang:1.24-alpine AS build

WORKDIR /app

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/api

# Production stage
FROM alpine:latest AS prod

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

# Copy the binary from build stage
COPY --from=build /app/main .
COPY --from=build /app/migrations ./migrations

# Expose port
EXPOSE 8032

# Run the binary
CMD ["./main"]
