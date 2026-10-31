# syntax=docker/dockerfile:1.7
FROM --platform=$BUILDPLATFORM golang:1.24.6-alpine@sha256:c8c5f95d64aa79b6547f3b626eb84b16a7ce18a139e3e9ca19a8c078b85ba80d AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ENV CGO_ENABLED=0 GOTOOLCHAIN=local

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    mkdir -p /out && \
    GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" \
    go build -mod=readonly -trimpath -buildvcs=false -ldflags="-s -w" -o /out/server ./cmd/server && \
    GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" \
    go build -mod=readonly -trimpath -buildvcs=false -ldflags="-s -w" -o /out/migrate ./cmd/migration

FROM alpine:3.19@sha256:6baf43584bcb78f2e5847d1de515f23499913ac9f12bdf834811a3145eb11ca1

ARG VCS_REF=unknown
ARG VCS_URL=https://github.com/SaRgEX98/corpord-backend
LABEL org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.source="${VCS_URL}"

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/server /out/migrate ./

COPY db/migrations ./db/migrations

COPY docs ./docs

RUN adduser -D -u 10001 appuser
USER 10001:10001

EXPOSE 8080

CMD ["./server"]
