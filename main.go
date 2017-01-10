package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/streadway/amqp"
)

var (
	uri          = flag.String("uri", "amqp://guest:guest@localhost:5672/", "AMQP URI")
	insecure_tls = flag.Bool("insecure-tls", false, "Insecure TLS mode: don't check certificates")
	queue        = flag.String("queue", "", "AMQP queue name")
        ack          = flag.Bool("ack", false, "Acknowledge messages")
	maxMessages  = flag.Uint("max-messages", 1000, "Maximum number of messages to dump")
	outputDir    = flag.String("output-dir", ".", "Directory in which to save the dumped messages")
	full         = flag.Bool("full", false, "Dump the message, its properties and headers")
	verbose      = flag.Bool("verbose", false, "Print progress")
)

func main() {
	flag.Parse()
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "Error: Unused command line arguments detected.\n")
		flag.Usage();
		os.Exit(2)
	}
	err := DumpMessagesFromQueue(*uri, *queue, *maxMessages, *outputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func dial(amqpURI string) (*amqp.Connection, error) {
	VerboseLog(fmt.Sprintf("Dialing %q", amqpURI))
	if *insecure_tls && strings.HasPrefix(amqpURI, "amqps://") {
		tlsConfig := new(tls.Config)
		tlsConfig.InsecureSkipVerify = true
		conn, err := amqp.DialTLS(amqpURI, tlsConfig)
		return conn, err
	}
	conn, err := amqp.Dial(amqpURI)
	return conn, err
}

func DumpMessagesFromQueue(amqpURI string, queueName string, maxMessages uint, outputDir string) error {
	if queueName == "" {
		return fmt.Errorf("Must supply queue name")
	}

	conn, err := dial(amqpURI)
	if err != nil {
		return fmt.Errorf("Dial: %s", err)
	}

	defer func() {
		conn.Close()
		VerboseLog("AMQP connection closed")
	}()

	channel, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("Channel: %s", err)
	}

	VerboseLog(fmt.Sprintf("Pulling messages from queue %q", queueName))
	for messagesReceived := uint(0); messagesReceived < maxMessages; messagesReceived++ {
		msg, ok, err := channel.Get(queueName,
			*ack, // autoAck
		)
		if err != nil {
			return fmt.Errorf("Queue get: %s", err)
		}

		if !ok {
			VerboseLog("No more messages in queue")
			break
		}

		err = SaveMessageToFile(msg.Body, outputDir, messagesReceived)
		if err != nil {
			return fmt.Errorf("Save message: %s", err)
		}

		if *full {
			err = SavePropsAndHeadersToFile(msg, outputDir, messagesReceived)
			if err != nil {
				return fmt.Errorf("Save props and headers: %s", err)
			}
		}
	}

	return nil
}

func SaveMessageToFile(body []byte, outputDir string, counter uint) error {
	filePath := GenerateFilePath(outputDir, counter)
	err := ioutil.WriteFile(filePath, body, 0644)
	if err != nil {
		return err
	}

	fmt.Println(filePath)

	return nil
}

func GetProperties(msg amqp.Delivery) map[string]interface{} {
	props := map[string]interface{}{
		"app_id":           msg.AppId,
		"content_encoding": msg.ContentEncoding,
		"content_type":     msg.ContentType,
		"correlation_id":   msg.CorrelationId,
		"delivery_mode":    msg.DeliveryMode,
		"expiration":       msg.Expiration,
		"message_id":       msg.MessageId,
		"priority":         msg.Priority,
		"reply_to":         msg.ReplyTo,
		"type":             msg.Type,
		"user_id":          msg.UserId,
	}

	if !msg.Timestamp.IsZero() {
		props["timestamp"] = msg.Timestamp.String()
	}

	for k, v := range props {
		if v == "" {
			delete(props, k)
		}
	}

	return props
}

func SavePropsAndHeadersToFile(msg amqp.Delivery, outputDir string, counter uint) error {
	extras := make(map[string]interface{})
	extras["properties"] = GetProperties(msg)
	extras["headers"] = msg.Headers

	data, err := json.MarshalIndent(extras, "", "  ")
	if err != nil {
		return err
	}

	filePath := GenerateFilePath(outputDir, counter) + "-headers+properties.json"
	err = ioutil.WriteFile(filePath, data, 0644)
	if err != nil {
		return err
	}

	fmt.Println(filePath)

	return nil
}

func GenerateFilePath(outputDir string, counter uint) string {
	return path.Join(outputDir, fmt.Sprintf("msg-%04d", counter))
}

func VerboseLog(msg string) {
	if *verbose {
		fmt.Println("*", msg)
	}
}
