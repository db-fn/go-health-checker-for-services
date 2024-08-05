FROM golang:1.20-alpine

# Установка зависимостей
RUN apk update && apk add --no-cache bash docker openrc

# Создание рабочего каталога
WORKDIR /app

# Копирование модулей и установка зависимостей
COPY go.mod ./
COPY go.sum ./

ENV GOPROXY=https://goproxy.io,direct

RUN go mod download

# Копирование исходного кода и сборка
COPY . .

RUN go build -o /healthcheck

# Открытие порта
EXPOSE 8000

# Установка PATH
ENV PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

# Команда запуска
CMD ["/healthcheck"]