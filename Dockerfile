FROM golang:1.12 as builder

# ARG project_id
# ENV project_id=${project_id}

WORKDIR /go/src/github.com/reallyasi9/beat-the-streak
COPY . .

RUN go get -d -v github.com/atgjack/prob github.com/segmentio/fasthash gopkg.in/yaml.v2 cloud.google.com/go/firestore firebase.google.com/go
RUN go install -v github.com/atgjack/prob github.com/segmentio/fasthash gopkg.in/yaml.v2 cloud.google.com/go/firestore firebase.google.com/go

RUN CGO_ENABLED=0 GOOS=linux go build -v -o bts-mc cmd/bts-mc/*.go

FROM alpine
RUN apk add --no-cache ca-certificates

COPY --from=builder /go/src/github.com/reallyasi9/beat-the-streak /bts-mc

CMD ["/bts-mc"]
# , "-project", ${project_id}