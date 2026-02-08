FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
ENV GOTOOLCHAIN=auto
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o k2api ./cmd/api

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    poppler-utils \
    tesseract-ocr \
    tesseract-ocr-swe \
    ocrmypdf \
  && rm -rf /var/lib/apt/lists/*

RUN groupadd -r app && useradd -r -g app app
WORKDIR /app

COPY --from=builder /src/k2api /app/k2api
COPY web /app/web

EXPOSE 8080
USER app

ENTRYPOINT ["/app/k2api"]
