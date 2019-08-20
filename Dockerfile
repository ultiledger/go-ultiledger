FROM golang:1.12-alpine as builder

RUN apk add --no-cache make

ADD . /go-ultiledger
RUN cd /go-ultiledger && build/env.sh go install

FROM alpine:latest

RUN apk add --no-cache ca-certificates

COPY --from=builder /go-ultiledger/build/bin/ult /usr/local/bin/

ENTRYPOINT ["ult"]
