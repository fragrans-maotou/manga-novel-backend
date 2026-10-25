# syntax=docker/dockerfile:1.7

# ---- build ----
FROM golang:1.23-alpine AS build
WORKDIR /src

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -trimpath -ldflags="-s -w" -o /out/manga-api ./cmd/api

# ---- runtime ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget \
  && addgroup -S app && adduser -S -G app app

WORKDIR /app
COPY --from=build /out/manga-api /app/manga-api

USER app
EXPOSE 8080
ENTRYPOINT ["/app/manga-api"]
