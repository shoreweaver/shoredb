FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -o /shoredb-server ./cmd/shoredb-server

FROM alpine:3.20
COPY --from=build /shoredb-server /usr/local/bin/shoredb-server
WORKDIR /data
EXPOSE 6379
ENTRYPOINT ["shoredb-server"]
CMD ["-port", ":6379", "-config", "/data/redis.config"]
