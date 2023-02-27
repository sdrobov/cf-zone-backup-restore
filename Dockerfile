FROM --platform=$BUILDPLATFORM golang:1.20-alpine AS builder

ARG CGO_ENABLED=0

WORKDIR /app

COPY ./go.mod ./go.mod
COPY ./go.sum ./go.sum
RUN go mod download

COPY ./main.go ./main.go
RUN go build -v -o cf-zone-backup ./main.go

FROM --platform=$BUILDPLATFORM alpine:latest

WORKDIR /app

COPY --from=builder /app/cf-zone-backup /app/cf-zone-backup

ENTRYPOINT '/app/cf-zone-backup'
