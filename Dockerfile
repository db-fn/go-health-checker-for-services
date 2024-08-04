FROM golang:1.20-alpine

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

ENV GOPROXY=https://goproxy.io,direct

RUN go mod download

COPY . .

RUN go build -o /healthcheck

EXPOSE 8000

CMD ["/healthcheck"]