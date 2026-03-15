FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o sync-board .

FROM alpine:latest

RUN apk add --no-cache ca-certificates ttf-freefont

WORKDIR /app

COPY --from=builder /app/sync-board .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/assets ./assets

EXPOSE 8000

CMD ["./sync-board"]
