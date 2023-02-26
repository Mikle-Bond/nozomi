FROM golang:1.19 as builder

WORKDIR /usr/src/app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o /nozomi ./...

FROM busybox

COPY --from=builder /nozomi /usr/bin/nozomi

CMD ["/usr/bin/nozomi"]

HEALTHCHECK --start-period=10s --interval=4m \
    CMD wget --tries=1 -qO - http://localhost:9000/health || exit 1
