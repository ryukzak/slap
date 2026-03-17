FROM golang:1.25-alpine AS builder

ARG VERSION=dev
ARG BUILD_TIME

WORKDIR /build

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags "-X main.version=${VERSION}-${BUILD_TIME}" -o slap main.go

FROM alpine:latest

RUN apk add --no-cache tzdata

WORKDIR /app

COPY --from=builder /build/slap .
COPY --from=builder /build/templates/ templates/
COPY --from=builder /build/static/ static/
COPY --from=builder /build/conf/ conf/

RUN adduser -D -h /app slap \
    && mkdir -p tmp \
    && chown -R slap:slap /app

USER slap

EXPOSE 8080

ENTRYPOINT ["./slap"]
