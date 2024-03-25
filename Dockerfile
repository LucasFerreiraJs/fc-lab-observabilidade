FROM golang:1.21.5 as build-main

WORKDIR /fc-lab-observabilidade

COPY cmd ./cmd
COPY internal ./internal
COPY go.mod ./go.mod
COPY go.mod ./go.mod

RUN go mod tidy
RUN go build -o main ./cmd/microservice/main.go
RUN ls


COPY main .

CMD ["./main"]