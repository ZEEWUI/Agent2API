FROM golang:1.23-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/agent2api ./cmd/agent2api

FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/agent2api /app/agent2api
COPY config.json /app/config.json
EXPOSE 8787
ENTRYPOINT ["/bin/sh","-c","exec /app/agent2api -config /app/config.json -host 0.0.0.0 -port ${PORT:-8787}"]
