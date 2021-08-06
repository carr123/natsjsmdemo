package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	nats "github.com/nats-io/nats.go"
)

var (
	servers     = []string{"nats://10.91.26.225:4222", "nats://10.91.26.226:4222", "nats://10.91.26.227:4222"}
	nats_user   = "your_user"
	nats_passwd = "your_passwd"
)

func main() {
	jsmRecv()
}

func jsmRecv() {
	defer func() {
		log.Println("exit recv")
	}()

	var err error

	nc, err := nats.Connect(
		strings.Join(servers, ","),
		nats.Name("jsmRecv"),
		nats.Timeout(10*time.Second),
		nats.PingInterval(20*time.Second),
		nats.MaxPingsOutstanding(5),
		nats.MaxReconnects(10),
		nats.ReconnectWait(10*time.Second),
		nats.ReconnectBufSize(5*1024*1024),
		nats.UserInfo(nats_user, nats_passwd),
	)
	if err != nil {
		log.Fatal(err)
		return
	}
	defer nc.Close()

	// Create JetStream Context
	js, err := nc.JetStream()
	if err != nil {
		log.Fatal(err)
		return
	}

	_, err = js.AddStream(&nats.StreamConfig{
		Name:      "ORDERS",
		Subjects:  []string{"ORDERS.*"},
		Storage:   nats.FileStorage,
		Replicas:  3,
		Retention: nats.WorkQueuePolicy,
		Discard:   nats.DiscardNew,
		MaxMsgs:   -1,
		MaxAge:    time.Hour * 24 * 365,
	})
	if err != nil && !strings.Contains(err.Error(), "already in use") {
		log.Println("2", err)
		return
	}

	durableName := "consume_created"

	//js.DeleteConsumer("ORDERS", durableName)

	if _, err := js.AddConsumer("ORDERS", &nats.ConsumerConfig{
		Durable:       durableName,
		DeliverPolicy: nats.DeliverAllPolicy,
		AckPolicy:     nats.AckExplicitPolicy,
		ReplayPolicy:  nats.ReplayInstantPolicy,
		FilterSubject: "ORDERS.created",
		AckWait:       time.Second * 30,
		MaxDeliver:    -1,
		MaxAckPending: 1000,
	}); err != nil && !strings.Contains(err.Error(), "already in use") {
		log.Println("AddConsumer fail:", err)
		return
	}

	fmt.Println("begin pull message")

	sub, err := js.PullSubscribe("ORDERS.created", durableName, nats.Bind("ORDERS", durableName))
	if err != nil {
		log.Println("PullSubscribe fail:", err)
		return
	}

	nTotal := 0
	for {
		if !sub.IsValid() {
			log.Println("sub invalid !!!!")
			return
		}

		msgs, err := sub.Fetch(1000, nats.MaxWait(5*time.Second))
		if err == nats.ErrNoResponders || err == nats.ErrTimeout {
			log.Println("Fetch fail 1:", err, len(msgs))
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if err != nil {
			log.Println(" Fetch fail 2:", err, len(msgs))
			return
		}

		for _, msg := range msgs {
			msg.Ack()
		}

		nTotal += len(msgs)
		log.Println("recv msgs:", len(msgs), " total:", nTotal)
	}
}
