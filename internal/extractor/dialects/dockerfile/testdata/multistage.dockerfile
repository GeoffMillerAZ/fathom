# syntax=docker/dockerfile:1
ARG GO_VERSION=1.21

FROM golang:${GO_VERSION} AS build
ARG VERSION=dev
ENV CGO_ENABLED=0
WORKDIR /src
COPY . .
RUN go build -o /out/app .

FROM build AS test
RUN go test ./...

FROM alpine:3.19 AS final
ENV APP_ENV=prod \
    APP_PORT=8080
COPY --from=build /out/app /usr/local/bin/app
CMD ["/usr/local/bin/app"]
