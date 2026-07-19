FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o sync-pad ./cmd/server

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/sync-pad /usr/local/bin/
COPY --from=builder /app/web /var/www/sync-pad
EXPOSE 8080
ENV SYNC_PAD_WEB_DIR=/var/www/sync-pad
CMD ["sync-pad"]
