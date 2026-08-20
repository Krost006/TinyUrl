# Один Dockerfile на оба сервиса: они отличаются только точкой входа.
# Какую собирать — задаёт build-arg TARGET (см. docker-compose.yml).
ARG TARGET=api

# Сборка отделена от запуска: в финальный образ не едут ни компилятор,
# ни исходники — только бинарник.
FROM golang:1.25-alpine AS build

ARG TARGET

WORKDIR /src

# Зависимости отдельным слоем: пока go.mod и go.sum не меняются, слой берётся
# из кеша и модули не скачиваются заново.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO выключен, чтобы бинарник был статическим: иначе он потребует libc той
# системы, где собирался, и не запустится в другом образе.
RUN CGO_ENABLED=0 go build -o /bin/service ./cmd/${TARGET}

FROM alpine:3.20

# Корневые сертификаты — задел на будущее: понадобятся, как только сервис
# начнёт делать исходящие HTTPS-запросы. Сейчас он никуда не ходит.
RUN apk add --no-cache ca-certificates

# Процессу в контейнере root не нужен: если в сервисе найдут дыру, атакующий
# окажется обычным пользователем. Порт 8080 больше 1024, так что прав хватает.
RUN adduser -D -u 10001 app
USER app

COPY --from=build /bin/service /bin/service

# Фронтенд нужен только сервису api, но копируем в оба образа: отдельный
# Dockerfile ради нескольких килобайт статики не окупается.
COPY web /web

EXPOSE 8080

ENTRYPOINT ["/bin/service"]
