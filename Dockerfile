FROM golang:1.16-alpine

RUN apk add git make nodejs npm gcc musl-dev linux-headers

WORKDIR /go/src/bakinbacon

RUN git clone https://github.com/bakingbacon/bakinbacon .

RUN make ui && make

VOLUME /var/db

EXPOSE 8082

ENTRYPOINT ["/go/src/bakinbacon/bakinbacon"]
