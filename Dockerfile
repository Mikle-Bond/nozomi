FROM golang:1.19 as builder

WORKDIR /usr/src/app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o /nozomi ./...

FROM busybox

COPY --from=builder /nozomi /usr/bin/nozomi

CMD ["/usr/bin/nozomi"]
