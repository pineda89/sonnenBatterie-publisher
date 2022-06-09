FROM golang AS builder

COPY . /go/src/application

WORKDIR /go/src/application/app

RUN go mod tidy
RUN go mod verify

RUN CGO_ENABLED=1 go build -o binary

FROM ubuntu:20.04

COPY --from=0 /go/src/application/app/binary .

CMD ["./binary"]