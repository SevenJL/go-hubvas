# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /ws-server ./cmd/ws-server

# Runtime stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /ws-server .
COPY configs/config.yaml ./configs/

EXPOSE 8081

ENTRYPOINT ["./ws-server"]
