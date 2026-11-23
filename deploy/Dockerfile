# syntax=docker/dockerfile:1

FROM golang:1.24-bookworm AS build
WORKDIR /src
ENV GOTOOLCHAIN=auto \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=v0.0.4
RUN go build -trimpath -ldflags="-s -w -X assethub/internal/version.Version=${VERSION}" -o /out/assethub .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
  && mkdir -p /data /app
WORKDIR /app

ENV ASSETHUB_EMBED_STATIC=1 \
    TZ=Asia/Shanghai

COPY --from=build /out/assethub /app/assethub
COPY config.docker.yaml /app/config.yaml

EXPOSE 8080
VOLUME ["/data"]
CMD ["/app/assethub"]
