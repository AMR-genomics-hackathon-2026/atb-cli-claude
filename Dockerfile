FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /atb ./cmd/atb

FROM alpine:3.20

RUN apk add --no-cache ca-certificates python3 py3-pyarrow

COPY --from=builder /atb /usr/local/bin/atb
COPY testdata/generate.py /opt/atb/generate_fixtures.py
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

VOLUME /data
EXPOSE 8080

ENV ATB_DATA_DIR=/data
ENV ATB_HTTP_PORT=8080

ENTRYPOINT ["docker-entrypoint.sh"]
