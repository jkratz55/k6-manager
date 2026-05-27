# Frontend build stage
FROM node:23-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm install
COPY frontend/ ./
RUN npm run build

# Build stage
FROM golang:1.26-alpine AS builder

# Set the working directory
WORKDIR /app

# Copy go.mod and go.sum files first to leverage Docker cache
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Copy the built frontend from the previous stage
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

# Build the application
# -ldflags="-s -w" strips debug information and symbol tables to reduce binary size
# CGO_ENABLED=0 ensures the binary is statically linked and can run in scratch/distroless
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o k6-manager main.go

# Final stage
# Use Google's distroless static image for minimal security footprint
# It contains only the minimal set of dependencies required to run a statically linked binary
FROM gcr.io/distroless/static-debian12

# Copy the binary from the builder stage
COPY --from=builder /app/k6-manager /k6-manager

# Use a non-root user (distroless has a 'nonroot' user with UID 65532)
USER 65532:65532

# Expose the port the application listens on
EXPOSE 8080

# Run the application
ENTRYPOINT ["/k6-manager"]
