FROM golang:1.25.1-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ws-server ./cmd/ws-server

FROM alpine:3.21
RUN apk --no-cache add ca-certificates tzdata && addgroup -S hubvas && adduser -S -G hubvas hubvas
WORKDIR /app
COPY --from=builder /out/ws-server ./
COPY configs/config.yaml ./configs/
USER hubvas
EXPOSE 8081
CMD ["./ws-server"]
