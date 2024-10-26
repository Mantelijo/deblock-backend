FROM golang:1.23-bookworm

WORKDIR /app
COPY . . 

RUN go mod download && go build -o /usr/bin/app ./cmd/main.go

CMD ["/usr/bin/app"]
