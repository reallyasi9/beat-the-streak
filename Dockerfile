FROM golang:1.12 as builder

WORKDIR /go/src/beat-the-streak
COPY . .

RUN go get -d -v github.com/atgjack/prob github.com/segmentio/fasthash gopkg.in/yaml.v2
RUN go install -v github.com/atgjack/prob github.com/segmentio/fasthash gopkg.in/yaml.v2

RUN CGO_ENABLED=0 GOOS=linux go build -v -o bts-mc mc/*.go

FROM alpine
RUN apk add --no-cache ca-certificates

COPY --from=builder /go/src/beat-the-streak /bts-mc

CMD ["/bts-mc"]