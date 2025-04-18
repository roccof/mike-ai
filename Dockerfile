FROM golang:latest

WORKDIR /usr/src/app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -v -o /usr/local/bin/app

COPY assets/ /var/www/mike-assets

EXPOSE 8080
CMD ["app"]
