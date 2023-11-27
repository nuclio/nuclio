package main

import (
	"fmt"
	"log"
	"time"

	"github.com/streadway/amqp"
)

const (
	rabbitMQURL = "amqp://guest:guest@localhost:5672/"
	exchange    = "element_exchange"
	priorityKey = "priority"
	timeKey     = "time"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func createChannel(conn *amqp.Connection) *amqp.Channel {
	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	return ch
}

func declareExchange(ch *amqp.Channel) {
	err := ch.ExchangeDeclare(
		exchange, // name
		"direct", // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
	failOnError(err, "Failed to declare the exchange")
}

func publishMessage(ch *amqp.Channel, key string, value int) {
	err := ch.Publish(
		exchange, // exchange
		key,      // routing key
		false,    // mandatory
		false,    // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(fmt.Sprintf("%d", value)),
		},
	)
	failOnError(err, "Failed to publish a message")
}

func consumeMessages(ch *amqp.Channel, queueName, attribute string) {
	q, err := ch.QueueDeclare(
		queueName, // name
		false,     // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	failOnError(err, "Failed to declare a queue")

	err = ch.QueueBind(
		q.Name,    // queue name
		attribute, // routing key
		exchange,  // exchange
		false,     // no-wait
		nil,       // arguments
	)
	failOnError(err, "Failed to bind a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			fmt.Printf("Received a message with %s: %s\n", attribute, d.Body)
			// Implement your logic to remove elements based on the attribute here
		}
	}()

	log.Printf(" [*] Waiting for messages with %s. To exit press CTRL+C", attribute)
	<-forever
}

func main() {
	conn, err := amqp.Dial(rabbitMQURL)
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch := createChannel(conn)
	defer ch.Close()

	declareExchange(ch)

	// Producer: Send messages with priority and time attributes
	for i := 1; i <= 5; i++ {
		publishMessage(ch, priorityKey, i)
		publishMessage(ch, timeKey, i)
		time.Sleep(1 * time.Second)
	}

	// Consumer: Consume messages based on priority
	go consumeMessages(ch, "priority_queue", priorityKey)

	// Consumer: Consume messages based on time
	go consumeMessages(ch, "time_queue", timeKey)

	select {}
}
