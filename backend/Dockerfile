FROM golang:1.26.4-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY docs ./docs

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/apiserver ./cmd/apiserver

FROM alpine:3.20.10

RUN adduser -D -H -u 10001 appuser

WORKDIR /app
COPY --from=builder /out/apiserver /app/apiserver

USER appuser
EXPOSE 8080

ENTRYPOINT ["/app/apiserver"]
