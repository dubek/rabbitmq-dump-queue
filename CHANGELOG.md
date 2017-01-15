# Change log

## Upcoming

* Print an error when there are unused arguments on the command-line.
* Add `-ack` option to acknowledge the received messages and therefore to
  *remove* them from the queue - from
  [@msteggink](https://github.com/msteggink).


## v0.3 (2016-11-01)

* Add system tests against a local RabbitMQ server.
* Add `-insecure-tls` option to skip verification of the RabbitMQ's TLS
  certificates; as the name hints, this is NOT SECURE.


## v0.2 (2016-07-01)

* Add `-full` option to dump the message header and properties into
  `msg-NNNN-headers+properties.json` files - from
  [@sshaw](https://github.com/sshaw).
* README clarifications - from [@kruppel](https://github.com/kruppel).
* Add amqp package is vendored using git submodule.


## v0.1 (2014-11-11)

* Initial release.
