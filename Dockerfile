FROM golang:1.27-alpine AS builder

RUN apk add --no-cache ca-certificates git tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/janusgate \
    ./cmd/gateway

FROM alpine:3.20 AS runner

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -S janusgroup && adduser -S janususer -G janusgroup

WORKDIR /app

COPY --from=builder /app/janusgate /app/janusgate
COPY config.yaml /app/config.yaml

RUN chown -R janususer:janusgroup /app

USER janususer

EXPOSE 8080 9090 6060

ENTRYPOINT ["/app/janusgate"]