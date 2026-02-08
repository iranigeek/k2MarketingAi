FROM golang:1.24-bookworm AS builder

WORKDIR /src
RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates \
  && rm -rf /var/lib/apt/lists/*
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app ./cmd/api

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    poppler-utils \
    tesseract-ocr \
    tesseract-ocr-swe \
    ocrmypdf \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /src/app /app/app
COPY --from=builder /src/web /app/web

EXPOSE 8080
CMD ["/app/app"]
