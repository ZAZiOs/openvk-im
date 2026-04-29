# --- STAGE 1: Build ---
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download


COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /ovk-im-server ./src/main.go

# --- STAGE 2: Run ---
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

COPY --from=builder /ovk-im-server .

COPY .env .env

EXPOSE 8080

CMD ["./ovk-im-server"]