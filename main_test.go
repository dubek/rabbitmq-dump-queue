package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/streadway/amqp"
)

const (
	testAmqpURI   = "amqp://guest:guest@127.0.0.1:5672/"
	testQueueName = "test-rabbitmq-dump-queue"
)

func makeAmqpMessage(i int) amqp.Publishing {
	headers := make(amqp.Table)
	headers["my-header"] = fmt.Sprintf("my-value-%d", i)
	return amqp.Publishing{
		Headers:     headers,
		ContentType: "text/plain",
		Priority:    4,
		MessageId:   fmt.Sprintf("msgid-%d", i),
		Body:        []byte(fmt.Sprintf("message-%d-body", i)),
	}
}

// Publish 10 messages to the queue
func populateTestQueue(t *testing.T, messagesToPublish int) {
	conn, err := amqp.Dial(testAmqpURI)
	if err != nil {
		t.Fatalf("Dial: %s", err)
	}
	defer conn.Close()

	channel, err := conn.Channel()
	if err != nil {
		t.Fatalf("Channel: %s", err)
	}

	_, err = channel.QueueDeclare(testQueueName, true, false, false, false, nil)
	if err != nil {
		t.Fatalf("QueueDeclare: %s", err)
	}

	_, err = channel.QueuePurge(testQueueName, false)
	if err != nil {
		t.Fatalf("QueuePurge: %s", err)
	}

	for i := 0; i < messagesToPublish; i++ {
		err = channel.Publish("", testQueueName, false, false, makeAmqpMessage(i))
		if err != nil {
			t.Fatalf("Publish: %s", err)
		}
	}
}

func deleteTestQueue(t *testing.T) {
	conn, err := amqp.Dial(testAmqpURI)
	if err != nil {
		t.Fatalf("Dial: %s", err)
	}
	defer conn.Close()

	channel, err := conn.Channel()
	if err != nil {
		t.Fatalf("Channel: %s", err)
	}

	_, err = channel.QueueDelete(testQueueName, false, false, false)
	if err != nil {
		t.Fatalf("QueueDelete: %s", err)
	}
}

func getTestQueueLength(t *testing.T) int {
	conn, err := amqp.Dial(testAmqpURI)
	if err != nil {
		t.Fatalf("Dial: %s", err)
	}
	defer conn.Close()

	channel, err := conn.Channel()
	if err != nil {
		t.Fatalf("Channel: %s", err)
	}

	queue, err := channel.QueueInspect(testQueueName)
	if err != nil {
		t.Fatalf("QueueInspect: %s", err)
	}

	return queue.Messages
}

func run(t *testing.T, commandLine string) string {
	queueLengthBeforeDump := getTestQueueLength(t)
	args := strings.Split(commandLine, " ")
	output, err := exec.Command("./rabbitmq-dump-queue", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %s: %s", err, string(output))
	}
	queueLengthAfterDump := getTestQueueLength(t)
	if queueLengthAfterDump != queueLengthBeforeDump {
		t.Errorf("Queue length changed after rabbitmq-dump-queue: expected %d but got %d", queueLengthBeforeDump, queueLengthAfterDump)
	}
	return string(output)
}

func verifyFileContent(t *testing.T, filename, expectedContent string) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatalf("Error reading %s: %s", filename, err)
	}
	if expectedContent != string(content) {
		t.Errorf("Wrong content for %s: expected '%s', got '%s'", filename, expectedContent, string(content))
	}
}

func TestAcknowledge(t *testing.T) {
	os.MkdirAll("tmp-test", 0775)
	defer os.RemoveAll("tmp-test")
	populateTestQueue(t, 10)
	defer deleteTestQueue(t)
	output, err := exec.Command("./rabbitmq-dump-queue", "-uri="+testAmqpURI, "-queue="+testQueueName, "-max-messages=3", "-output-dir=tmp-test", "-ack=true").CombinedOutput()
	if err != nil {
		t.Fatalf("run: %s: %s", err, string(output))
	}
	expectedOutput := "tmp-test/msg-0000\n" +
		"tmp-test/msg-0001\n" +
		"tmp-test/msg-0002\n"
	if string(output) != expectedOutput {
		t.Errorf("Wrong output: expected '%s' but got '%s'", expectedOutput, output)
	}
	output2, err2 := exec.Command("./rabbitmq-dump-queue", "-uri="+testAmqpURI, "-queue="+testQueueName, "-max-messages=10", "-output-dir=tmp-test", "-ack=true").CombinedOutput()
	if err2 != nil {
		t.Fatalf("run: %s: %s", err, string(output))
	}
	expectedOutput2 := "tmp-test/msg-0000\n" +
		"tmp-test/msg-0001\n" +
		"tmp-test/msg-0002\n" +
		"tmp-test/msg-0003\n" +
		"tmp-test/msg-0004\n" +
		"tmp-test/msg-0005\n" +
		"tmp-test/msg-0006\n"
	if string(output2) != expectedOutput2 {
		t.Errorf("Wrong output: expected '%s' but got '%s'", expectedOutput2, output2)
	}
}

func TestNormal(t *testing.T) {
	os.MkdirAll("tmp-test", 0775)
	defer os.RemoveAll("tmp-test")
	populateTestQueue(t, 10)
	defer deleteTestQueue(t)
	output := run(t, "-uri="+testAmqpURI+" -queue="+testQueueName+" -max-messages=3 -output-dir=tmp-test")
	expectedOutput := "tmp-test/msg-0000\n" +
		"tmp-test/msg-0001\n" +
		"tmp-test/msg-0002\n"
	if output != expectedOutput {
		t.Errorf("Wrong output: expected '%s' but got '%s'", expectedOutput, output)
	}
	verifyFileContent(t, "tmp-test/msg-0000", "message-0-body")
	verifyFileContent(t, "tmp-test/msg-0001", "message-1-body")
	verifyFileContent(t, "tmp-test/msg-0002", "message-2-body")
	_, err := os.Stat("tmp-test/msg-0003")
	if !os.IsNotExist(err) {
		t.Errorf("Expected msg-0003 to not exist: %v", err)
	}
}

func TestEmptyQueue(t *testing.T) {
	os.MkdirAll("tmp-test", 0775)
	defer os.RemoveAll("tmp-test")
	populateTestQueue(t, 0)
	defer deleteTestQueue(t)
	output := run(t, "-uri="+testAmqpURI+" -queue="+testQueueName+" -max-messages=3 -output-dir=tmp-test")
	expectedOutput := ""
	if output != expectedOutput {
		t.Errorf("Wrong output: expected '%s' but got '%s'", expectedOutput, output)
	}
}

func TestMaxMessagesLargerThanQueueLength(t *testing.T) {
	os.MkdirAll("tmp-test", 0775)
	defer os.RemoveAll("tmp-test")
	populateTestQueue(t, 3)
	defer deleteTestQueue(t)
	output := run(t, "-uri="+testAmqpURI+" -queue="+testQueueName+" -max-messages=9 -output-dir=tmp-test")
	expectedOutput := "tmp-test/msg-0000\n" +
		"tmp-test/msg-0001\n" +
		"tmp-test/msg-0002\n"
	if output != expectedOutput {
		t.Errorf("Wrong output: expected '%s' but got '%s'", expectedOutput, output)
	}
}

func TestFull(t *testing.T) {
	os.MkdirAll("tmp-test", 0775)
	defer os.RemoveAll("tmp-test")
	populateTestQueue(t, 10)
	defer deleteTestQueue(t)
	output := run(t, "-uri="+testAmqpURI+" -queue="+testQueueName+" -max-messages=3 -output-dir=tmp-test -full")
	expectedOutput := "tmp-test/msg-0000\n" +
		"tmp-test/msg-0000-headers+properties.json\n" +
		"tmp-test/msg-0001\n" +
		"tmp-test/msg-0001-headers+properties.json\n" +
		"tmp-test/msg-0002\n" +
		"tmp-test/msg-0002-headers+properties.json\n"
	if output != expectedOutput {
		t.Errorf("Wrong output: expected '%s' but got '%s'", expectedOutput, output)
	}
	verifyFileContent(t, "tmp-test/msg-0000", "message-0-body")
	jsonContent, err := ioutil.ReadFile("tmp-test/msg-0000-headers+properties.json")
	if err != nil {
		t.Fatalf("Error reading tmp-test/msg-0000-headers+properties.json: %s", err)
	}
	var v map[string]interface{}
	err = json.Unmarshal(jsonContent, &v)
	if err != nil {
		t.Fatalf("Error unmarshaling JSON: %s", err)
	}

	headers, ok := v["headers"].(map[string]interface{})
	if !ok {
		t.Fatalf("Wrong data type for 'headers' in JSON")
	}
	if headers["my-header"] != "my-value-0" {
		t.Errorf("Wrong value for my-header, got: %v", headers["my-header"])
	}

	properties, ok := v["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("Wrong data type for 'properties' in JSON")
	}
	if properties["priority"] != 4.0 || // JSON numbers are floats
		properties["content_type"] != "text/plain" ||
		properties["message_id"] != "msgid-0" {
		t.Errorf("Wrong property value: properties = %#v", properties)
	}
}
