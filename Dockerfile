# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
WORKDIR /src/web

COPY web/package.json web/yarn.lock web/.yarnrc.yml ./
RUN corepack enable && yarn install --non-interactive

COPY web/ ./
RUN yarn build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN rm -rf internal/web/dist \
    && mkdir -p internal/web/dist
COPY --from=frontend /src/web/dist ./internal/web/dist

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/kohame ./cmd/kohame

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S kohame \
    && adduser -S -G kohame -h /var/lib/kohame kohame

WORKDIR /var/lib/kohame
COPY --from=builder /out/kohame /usr/local/bin/kohame
COPY config.yml ./config.yml

RUN mkdir -p data/repos \
    && chown -R kohame:kohame /var/lib/kohame

USER kohame

EXPOSE 3000
VOLUME ["/var/lib/kohame/data"]

ENTRYPOINT ["kohame"]
CMD ["-config", "/var/lib/kohame/config.yml"]
