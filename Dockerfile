FROM golang:1.23-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /server ./cmd/server

FROM alpine:3.20
RUN printf 'https://mirror.yandex.ru/mirrors/alpine/v3.20/main\nhttps://mirror.yandex.ru/mirrors/alpine/v3.20/community\n' > /etc/apk/repositories \
    && apk update \
    && apk add --no-cache ca-certificates wget tzdata \
    && adduser -D -u 1000 app
COPY --from=build /server /usr/local/bin/server
USER app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/server"]
