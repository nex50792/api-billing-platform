FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o bin/server ./cmd/server/main.go

FROM alpine:3.23

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/bin/server /app/server

EXPOSE 8080

CMD ["./server"]
