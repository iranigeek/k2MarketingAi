FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
ENV GOTOOLCHAIN=auto
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o k2api ./cmd/api

FROM alpine:3.20

RUN addgroup -S app && adduser -S app -G app
WORKDIR /app

COPY --from=builder /src/k2api /app/k2api
COPY web /app/web

EXPOSE 8080
USER app

ENTRYPOINT ["/app/k2api"]
