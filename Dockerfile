FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/openspeedtestx ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates && \
    adduser -D -u 10001 appuser

WORKDIR /app

COPY --from=build /out/openspeedtestx /usr/local/bin/openspeedtestx
COPY index.html hosted.html downloading upload License.md README.md ./
COPY assets ./assets

RUN mkdir -p /app/data && chown -R appuser:appuser /app

USER appuser

EXPOSE 3000
VOLUME ["/app/data"]

ENTRYPOINT ["openspeedtestx"]
CMD ["-addr", ":3000", "-root", "/app", "-data-file", "/app/data/history.json"]
