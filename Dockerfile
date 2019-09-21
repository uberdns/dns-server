FROM golang:1.13

RUN apt update

ENV GOPATH=/go

EXPOSE 53

WORKDIR /root

COPY . /root

RUN go build

ENTRYPOINT ["/root/dns-server"]
