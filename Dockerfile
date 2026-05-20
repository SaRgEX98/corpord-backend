FROM golang:1.24.6-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

# код
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o server ./cmd/server

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o migrate ./cmd/migration

FROM alpine:3.19

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/server .
COPY --from=builder /app/migrate .

COPY configs ./configs
COPY db/migrations ./db/migrations

COPY docs ./docs

RUN ls -la /app
RUN ls -la /app/configs
RUN adduser -D appuser
USER appuser

EXPOSE 8080

CMD ["./server"]