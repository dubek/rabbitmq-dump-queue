package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/rabbitmq/amqp091-go"
)

const headersAndPropertiesSuffix = "-headers+properties"

var (
	uri         = flag.String("uri", "amqp://guest:guest@localhost:5672/", "AMQP URI")
	insecureTLS = flag.Bool("insecure-tls", false, "Insecure TLS mode: don't check certificates")
	queue       = flag.String("queue", "", "AMQP queue name")
	ack         = flag.Bool("ack", false, "Acknowledge messages")
	maxMessages = flag.Uint("max-messages", 1000, "Maximum number of messages to dump or 0 for unlimited")
	outputDir   = flag.String("output-dir", ".", "Directory in which to save the dumped messages")
	singleFile  = flag.Bool("single-file", false, "Dump all messages to a single unified file")
	full        = flag.Bool("full", false, "Dump the message, its properties and headers")
	verbose     = flag.Bool("verbose", false, "Print progress")
)

func main() {
	flag.Parse()
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "Error: Unused command line arguments detected.\n")
		flag.Usage()
		os.Exit(2)
	}
	err := os.MkdirAll(*outputDir, 0644)
	if err == nil {
		err = dumpMessagesFromQueue(*uri, *queue, *maxMessages, *outputDir, *singleFile)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func dial(amqpURI string) (*amqp091.Connection, error) {
	verboseLog(fmt.Sprintf("Dialing %q", amqpURI))
	if *insecureTLS && strings.HasPrefix(amqpURI, "amqps://") {
		tlsConfig := new(tls.Config)
		tlsConfig.InsecureSkipVerify = true
		conn, err := amqp091.DialTLS(amqpURI, tlsConfig)
		return conn, err
	}
	conn, err := amqp091.Dial(amqpURI)
	return conn, err
}

func dumpMessagesFromQueue(amqpURI string, queueName string, maxMessages uint, outputDir string, singleFile bool) error {
	if queueName == "" {
		return fmt.Errorf("Must supply queue name")
	}

	conn, err := dial(amqpURI)
	if err != nil {
		return fmt.Errorf("Dial: %s", err)
	}

	defer func() {
		conn.Close()
		verboseLog("AMQP connection closed")
	}()

	channel, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("Channel: %s", err)
	}

	verboseLog(fmt.Sprintf("Pulling messages from queue %q", queueName))

	if singleFile {
		err = writeToSingleFiles(outputDir, false, "[\n")
		if err != nil {
			return err
		}
	}

	for messagesReceived := uint(0); maxMessages == 0 || messagesReceived < maxMessages; messagesReceived++ {
		msg, ok, err := channel.Get(queueName,
			*ack, // autoAck
		)
		if err != nil {
			return fmt.Errorf("Queue get: %s", err)
		}

		if !ok {
			verboseLog("No more messages in queue")
			break
		}

		if messagesReceived > 0 && singleFile {
			err = writeToSingleFiles(outputDir, true, ",\n")
			if err != nil {
				return err
			}
		}

		err = saveMessageToFile(msg.Body, outputDir, singleFile, messagesReceived)
		if err != nil {
			return fmt.Errorf("Save message: %s", err)
		}

		if *full {
			err = savePropsAndHeadersToFile(msg, outputDir, singleFile, messagesReceived)
			if err != nil {
				return fmt.Errorf("Save props and headers: %s", err)
			}
		}
	}

	if singleFile {
		err = writeToSingleFiles(outputDir, true, "\n]")
		if err != nil {
			return err
		}
	}

	return nil
}

func writeToSingleFiles(outputDir string, append bool, content string) error {
	err := writeToFile(generateFilePath(outputDir, true, 0, ""), []byte(content), append)
	if err != nil {
		return fmt.Errorf("Failed to write file: %s", err)
	}
	if *full {
		err = writeToFile(generateFilePath(outputDir, true, 0, headersAndPropertiesSuffix), []byte("[\n"), append)
		if err != nil {
			return fmt.Errorf("Failed to write file: %s", err)
		}
	}
	return nil
}

func saveMessageToFile(body []byte, outputDir string, singleFile bool, counter uint) error {
	filePath := generateFilePath(outputDir, singleFile, counter, "")
	err := writeToFile(filePath, body, singleFile)
	if err != nil {
		return err
	}

	if !singleFile || counter == 0 {
		fmt.Println(filePath)
	}

	return nil
}

func writeToFile(filePath string, body []byte, append bool) error {
	openFlags := os.O_WRONLY | os.O_CREATE
	if append {
		openFlags |= os.O_APPEND
	} else {
		openFlags |= os.O_TRUNC
	}
	f, err := os.OpenFile(filePath, openFlags, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(body)
	if err != nil {
		return err
	}
	return nil
}

func getProperties(msg amqp091.Delivery) map[string]interface{} {
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
		"exchange":         msg.Exchange,
		"routing_key":      msg.RoutingKey,
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

func savePropsAndHeadersToFile(msg amqp091.Delivery, outputDir string, singleFile bool, counter uint) error {
	extras := make(map[string]interface{})
	extras["properties"] = getProperties(msg)
	extras["headers"] = msg.Headers

	data, err := json.MarshalIndent(extras, "", "  ")
	if err != nil {
		return err
	}

	filePath := generateFilePath(outputDir, singleFile, counter, headersAndPropertiesSuffix)
	err = writeToFile(filePath, data, singleFile)
	if err != nil {
		return err
	}

	if !singleFile || counter == 0 {
		fmt.Println(filePath)
	}

	return nil
}

func generateFilePath(outputDir string, singleFile bool, counter uint, suffix string) string {
	fileName := "messages"
	if !singleFile {
		fileName = fmt.Sprintf("msg-%04d", counter)
	}
	if suffix != "" {
		fileName += suffix
	}
	fileName += ".json"
	return path.Join(outputDir, fileName)
}

func verboseLog(msg string) {
	if *verbose {
		fmt.Println("*", msg)
	}
}
