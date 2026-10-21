FROM golang:1.27.2 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/app ./cmd/app
FROM alpine:3.23
COPY --from=build /out/app /app
EXPOSE 8080
ENTRYPOINT ["/app"]
