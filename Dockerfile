FROM golang:1.21.5 as build-main

WORKDIR /app

COPY cmd ./cmd
COPY internal ./internal
COPY go.mod .

RUN go mod tidy
RUN go build -o main ./cmd/microservice/main.go

COPY internal/web/template template
RUN ls

CMD ["./main"]