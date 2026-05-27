# Frontend build stage
FROM node:23-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm install
COPY frontend/ ./
RUN npm run build

# Backend build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o k6-manager main.go

# Final stage
FROM gcr.io/distroless/static-debian12

# Copy the binary from the builder stage
COPY --from=builder /app/k6-manager /k6-manager

# Use a non-root user (distroless has a 'nonroot' user with UID 65532)
USER 65532:65532

# Expose the port the application listens on
EXPOSE 8080

# Run the application
ENTRYPOINT ["/k6-manager"]
