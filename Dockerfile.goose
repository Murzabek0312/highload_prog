FROM golang:1.27.2 AS build
RUN CGO_ENABLED=0 go install github.com/pressly/goose/v3/cmd/goose@v3.29.0

FROM alpine:3.23

COPY --from=build /go/bin/goose /usr/local/bin/goose
COPY migrations /migrations

ENTRYPOINT ["goose", "-dir", "/migrations"]