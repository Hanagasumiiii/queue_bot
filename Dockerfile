FROM golang:latest

WORKDIR /app

COPY queueBot.go .

# dependencies
COPY ["go.mod", "go.sum", "./"]
RUN go mod download

RUN go build -o ./bin/app queueBot.go

ENTRYPOINT ["bin/app"]