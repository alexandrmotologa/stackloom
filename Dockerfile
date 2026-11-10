# Stage 1: Build the React 19 Frontend
FROM node:22-alpine AS web-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build the Go Single Binary
FROM golang:1.24-alpine AS go-builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /app/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/stackloom ./cmd/stackloom

# Stage 3: Minimal Runtime Image (< 20MB)
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /workspace
COPY --from=go-builder /app/bin/stackloom /usr/local/bin/stackloom

EXPOSE 9095
ENTRYPOINT ["/usr/local/bin/stackloom"]
CMD ["serve", "-p", "9095", "-host", "0.0.0.0"]
