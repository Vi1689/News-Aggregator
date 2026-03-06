FROM golang:alpine AS builder

WORKDIR /build

ADD ./researchers/vk/go.mod .

COPY ./researchers/vk .

RUN go build .

FROM alpine

COPY --from=builder /build/researcher-vk /usr/local/bin/researcher-vk

RUN chmod +x /usr/local/bin/researcher-vk

CMD ["/usr/local/bin/researcher-vk"]