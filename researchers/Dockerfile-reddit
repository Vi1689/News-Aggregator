FROM golang:alpine AS builder

WORKDIR /build

ADD ./researchers/reddit/go.mod .

COPY ./researchers/reddit .

RUN go build .

FROM alpine

COPY --from=builder /build/researcher-reddit /usr/local/bin/researcher-reddit

RUN chmod +x /usr/local/bin/researcher-reddit

CMD ["/usr/local/bin/researcher-reddit"]
