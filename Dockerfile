FROM golang:1.24-bookworm AS builder
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/nesd ./cmd/nesd

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg ca-certificates tini \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /out/nesd /usr/local/bin/nesd
COPY docker/config.json /app/config.json
RUN mkdir -p /data/hls
EXPOSE 18080
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/nesd", "serve", "--config", "/app/config.json"]
