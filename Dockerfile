# Use the official Golang image to create a build artifact.
FROM golang:1.24.3-alpine as builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source code
COPY . .

# Build the application
# -ldflags="-w -s" reduces the size of the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /go/bin/app ./sample

# Use a small base image to create the final production image
FROM alpine:latest

WORKDIR /

# Copy the binary from the builder stage
COPY --from=builder /go/bin/app /app

# Expose port 8080 to the outside world
EXPOSE 8080

# Command to run the executable
ENTRYPOINT ["/app"]
