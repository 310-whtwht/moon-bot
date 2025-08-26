FROM golang:1.24-alpine

WORKDIR /app

# Install git, ca-certificates, and curl
RUN apk add --no-cache git ca-certificates curl

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o main .

# Expose port
EXPOSE 8081

# Start the application
CMD ["./main"]
