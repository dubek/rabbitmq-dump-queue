# rabbitmq-dump-queue
Dump messages from a RabbitMQ queue to files, without affecting the queue.

[![build-and-test](https://github.com/dubek/rabbitmq-dump-queue/actions/workflows/build-and-test.yml/badge.svg)](https://github.com/dubek/rabbitmq-dump-queue/actions/workflows/build-and-test.yml)

## Installation

### Download a release

Precompiled binary packages can be found on the
[releases](https://github.com/dubek/rabbitmq-dump-queue/releases) page.

### Compile from source

If you have [Go](https://golang.org/doc/install) installed, you can install
rabbitmq-dump-queue from source by running:

```
go install github.com/dubek/rabbitmq-dump-queue
```

The `rabbitmq-dump-queue` executable will be created in the `$GOPATH/bin`
directory.

### Compile in docker

If you want to use the application within a root-less container use `scripts/build_container`

``` bash
$ scripts/build_container -h
Usage: build_container [OPTIONS]

Options:
  -b, --builder <builder>  Specify the builder (docker or kaniko).
  -t, --target <target>    Specify the target (build, test, debug or production).
                           Default: production.
  -g, --tag <tag>          Specify the tag.
                           Default: latest for production target or <target>.
  -p, --dry-run            Print only mode.
  -h, --help               Display this help message.

Examples:
  build_container -b docker -t my-image --tag latest
  build_container --builder kaniko --target debug

```

## Usage

To dump the first 50 messages of queue `incoming_1` to `/tmp`:

    rabbitmq-dump-queue -uri="amqp://user:password@rabbitmq.example.com:5672/" -queue=incoming_1 -max-messages=50 -output-dir=/tmp

This will create the files `/tmp/msg-0000`, `/tmp/msg-0001`, and so on.

If the queue is in a RabbitMQ vhost, you should add the vhost name to the end
of the URI:

    rabbitmq-dump-queue -uri="amqp://user:password@rabbitmq.example.com:5672/vhost-name" ...

If the vhost name starts with `/` you'll need to specify it explicitly (double
slash after the port number).

The output filenames are printed one per line to the standard output; this
allows piping the output of rabbitmq-dump-queue to `xargs` or similar utilities
in order to perform further processing on each message (e.g. decompressing,
decoding, etc.).  For example:

    # Count the number of lines in each message
    rabbitmq-dump-queue -output-dir=/some/dir | xargs wc -l

    # Add messsages from the queue to a tar archive
    rabbitmq-dump-queue -output-dir=/some/dir | tar -czf /some/archive.tgz --remove-files -T -

To include the AMQP headers and properties in the output, add the `-full`
option to the command-line.  This will create the following files:

    /tmp/msg-0000
    /tmp/msg-0000-headers+properties.json
    /tmp/msg-0001
    /tmp/msg-0001-headers+properties.json
    ...

The JSON files have the following structure:

    {
      "headers": {
        "x-my-private-header": "my-value"
      },
      "properties": {
        "correlation_id": "XYZ-9876",
        "delivery_mode": 0,
        "priority": 5
      }
    }


By default, it will not acknowledge messages, so they will be requeued.
Acknowledging messages using the `-ack=true` switch will *remove* them from the
queue, allowing the user to process new messages (see implementation details).

    rabbitmq-dump-queue -uri="amqp://user:password@rabbitmq.example.com:5672/" -queue=incoming_1 -max-messages=50 -output-dir=/tmp -ack=true

Running `rabbitmq-dump-queue -help` will list the available command-line
options.



## Message requeuing implementation details

In order to fetch messages from the queue and later return them in the original
order, rabbitmq-dump-queue uses a standard [AMQP `basic.get` API
call](https://www.rabbitmq.com/amqp-0-9-1-reference.html#basic.get) without
automatic acknowledgements, and it doesn't manually acknowledge the received
messages.  Thus, when the AMQP connection is closed (after all the messages
were received and written to files), RabbitMQ returns all the un-acked messages
(all the messages) back to the queue in their original order.

This means that during the time rabbitmq-dump-queue receives and saves the
messages, the messages are not visible to other consumers of the queue.  This
duration is usually very short (unless you're downloading a lot of messages),
but make sure your system can handle such a situation (or shut down other
consumers of the queue during the time you use this tool).

Note that the same approach is used by RabbitMQ's management HTTP API (the
`/api/queues/{vhost}/{queue}/get` endpoint with `requeue=true`).


## Testing

To run the automated tests, have a RabbitMQ server listen on 127.0.0.1:5672
with user `guest` and password `guest`.  This can be easily achieved with
Docker:

    docker run --name test-rabbitmq -d -p 5672:5672 rabbitmq:3-management

Then run:

    go build .
    go test -v .

To stop the RabbitMQ server container, run:

    docker stop test-rabbitmq
    docker rm test-rabbitmq

Testing with `docker` can simpify the task, simply run:

    bash scripts/test

## Contributing

Github pull requests and issues are welcome.


## License

rabbitmq-dump-queue is under the MIT License. See the [LICENSE](LICENSE) file
for details.
