package main

import (
	"flag"
	"fmt"
	"github.com/streadway/amqp"
	"io/ioutil"
	"os"
	"path"
)

var (
	uri         = flag.String("uri", "amqp://guest:guest@localhost:5672/", "AMQP URI")
	queue       = flag.String("queue", "", "Ephemeral AMQP queue name")
	maxMessages = flag.Uint("max-messages", 1000, "Maximum number of messages to dump")
	outputDir   = flag.String("output-dir", ".", "Directory in which to save the dumped messages")
	verbose     = flag.Bool("verbose", false, "Print progress")
)

func main() {
	flag.Parse()
	err := DumpMessagesFromQueue(*uri, *queue, *maxMessages, *outputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func DumpMessagesFromQueue(amqpURI string, queueName string, maxMessages uint, outputDir string) error {
	var err error

	if queueName == "" {
		return fmt.Errorf("Must supply queue name")
	}

	VerboseLog(fmt.Sprintf("Dialing %q", amqpURI))
	conn, err := amqp.Dial(amqpURI)
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
			false, // autoAck
		)
		if err != nil {
			return fmt.Errorf("Queue Get: %s", err)
		}
		if ok {
			SaveMessageToFile(msg.Body, outputDir, messagesReceived)
		} else {
			VerboseLog("No more messages in queue")
			break
		}
	}

	return nil
}

func SaveMessageToFile(body []byte, outputDir string, counter uint) {
	filePath := GenerateFilePath(outputDir, counter)
	ioutil.WriteFile(filePath, body, 0644)
	fmt.Println(filePath)
}

func GenerateFilePath(outputDir string, counter uint) string {
	return path.Join(outputDir, fmt.Sprintf("msg-%04d", counter))
}

func VerboseLog(msg string) {
	if *verbose {
		fmt.Println("*", msg)
	}
}
