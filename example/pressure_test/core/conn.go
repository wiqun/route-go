package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/wiqun/route-go"
	routeMessage "github.com/wiqun/route-go/message"
	"time"
)

//type sendContent struct {
//	topic string
//	data  []byte
//}

type Conn struct {
	conn routego.Client

	SubChan   chan string
	unsubChan chan string
	sendChan  chan []string

	RecvTotal int64
	RecvCount int64

	SendCount int64
}

type FmtLogger struct {
}

func (f FmtLogger) Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func (f FmtLogger) Println(v ...interface{}) {
	fmt.Println(v...)
}

func (f FmtLogger) PrintlnError(v ...interface{}) {
	fmt.Println(v...)
}

func (f FmtLogger) PrintfError(format string, v ...interface{}) {
	fmt.Printf(format, v...)
}

func NewConn(addr string, size int) *Conn {
	c := &Conn{
		SubChan:   make(chan string, size),
		unsubChan: make(chan string, size),
		sendChan:  make(chan []string, size),
	}

	options := routego.NewDefaultOption(addr)
	options.QueryChanSize = size
	options.PubChanSize = size
	options.TopicOpChanSize = size

	client, err := routego.NewClient(options, FmtLogger{})
	if err != nil {
		panic(err)
	}
	client.SetOnMessageHandler(c.processRecv)

	err = client.Connect()
	if err != nil {
		panic(err)
	}
	c.conn = client

	return c

}

func (c *Conn) Run() {
	go c.processSend()
}

func (c *Conn) processSend() {
	for {
		select {
		case sub := <-c.SubChan:
			e := c.conn.BatchSubscribe([]string{sub})
			if e != nil {
				panic(e)
			}
		case unsub := <-c.unsubChan:
			e := c.conn.BatchSubscribe([]string{unsub})
			if e != nil {
				panic(e)
			}
		case topicList := <-c.sendChan:

			Batch := make([]*routeMessage.PubRequest, 0, len(topicList))
			payload := IntToBytes(time.Now().UnixNano())

			for _, topic := range topicList {
				Batch = append(Batch, &routeMessage.PubRequest{Topic: topic, Payload: payload})
			}

			e := c.conn.BatchPublish(Batch)
			if e != nil {
				panic(e)
			}
			c.SendCount++
		}
	}
}

func (c *Conn) processRecv(messages []*routeMessage.Message) {

	for _, pub := range messages {
		startTime := time.Unix(0, BytesToInt(pub.Payload))
		t := time.Now().Sub(startTime)

		c.RecvTotal += t.Milliseconds()
		c.RecvCount++
	}

}

func (c *Conn) Sub(topic string) {
	c.SubChan <- topic
}

func (c *Conn) Unsub(topic string) {
	c.unsubChan <- topic

}

func (c *Conn) Send(topic []string) {
	c.sendChan <- topic
}

//整形转换成字节
func IntToBytes(n int64) []byte {
	x := n
	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.BigEndian, x)
	return bytesBuffer.Bytes()
}

//字节转换成整形
func BytesToInt(b []byte) int64 {
	bytesBuffer := bytes.NewBuffer(b)

	var x int64
	binary.Read(bytesBuffer, binary.BigEndian, &x)
	return x
}
