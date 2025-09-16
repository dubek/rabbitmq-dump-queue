ARG BASE_IMAGE=gcr.io/distroless/static

# build stage
FROM golang:alpine AS build

COPY go* main* .

RUN apk add --no-cache git

RUN CGO_ENABLED=0 go build -o rabbitmq-dump-queue .


# test stage
FROM build AS test

ENV GOPATH=''
ENTRYPOINT [ "go", "test" ]


# production stage
FROM ${BASE_IMAGE} AS production
ARG UID=65532
ARG GID=65532

# make latest alpine certs available
COPY --from=alpine:latest /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

USER ${UID}:${GID}

# copy app binary
COPY --from=build /go/rabbitmq-dump-queue /usr/local/bin/

ENTRYPOINT [ "rabbitmq-dump-queue" ]

# volume dir to output data
VOLUME /data
WORKDIR /data


# degug stage
FROM busybox:stable-uclibc AS busybox
FROM production AS debug

COPY --from=busybox /bin/id /bin/sh /bin/busybox /bin/


# default stage: production
FROM production
