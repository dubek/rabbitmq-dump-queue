# build stage
FROM golang:alpine AS build
ARG UID=1000

COPY go* main* .

RUN apk add --no-cache git
RUN adduser -u ${UID} -D -h /app -H scratchuser

RUN CGO_ENABLED=0 go build -ldflags '-extldflags "-static"' -o bin/rabbitmq-dump-queue .


# test stage
FROM build AS test

ENV GOPATH=''
ENTRYPOINT [ "go", "test" ]


# production stage
FROM scratch AS production

# copy root-less user
COPY --from=build /etc/passwd /etc/passwd
# make latest alpine certs available
COPY --from=alpine:latest /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

USER scratchuser

# copy app binary
COPY --from=build /go/bin/rabbitmq-dump-queue /usr/local/bin/

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
