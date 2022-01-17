FROM golang:alpine as builder
RUN apk add --update --no-cache ca-certificates
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN go get
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o app .

FROM scratch
COPY --from=builder /build/app /vibeinfo/
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
WORKDIR /vibeinfo
CMD ["./app"]
