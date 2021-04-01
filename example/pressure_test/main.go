package main

import (
	"bufio"
	"fmt"
	"github.com/wiqun/route-go/example/pressure_test/core"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

type client struct {
	c        *core.Conn
	subList  []string
	sendList []string
	addr     string
	size     int
}

func (c *client) creatConn() {
	c.c = core.NewConn(c.addr, c.size)
}

func (c *client) runConn() {
	c.c.Run()
}

func (c *client) doSub(wg *sync.WaitGroup) {
	defer wg.Done()
	for _, topic := range c.subList {
		c.c.Sub(topic)
	}

	ticker := time.NewTicker(time.Second)
	for {
		<-ticker.C
		if len(c.c.SubChan) == 0 {
			ticker.Stop()
			return
		}
	}

}

func (c *client) doSend(sendInterval time.Duration, enableBatch bool) {
	core.NewSender(c.c, c.sendList, sendInterval).Run(enableBatch)
}

func main() {
	go func() {
		log.Println(http.ListenAndServe(":6061", nil))
	}()
	//需要压测的服务器地址
	addr := "ws://127.0.0.1:4000"

	sendInterval := time.Millisecond * 270
	flushInterval := time.Second
	aggregationInterval := time.Second

	//连接数量
	connTotal := 3000

	//每次发送的数量
	sendRepeat := 1

	//是否启用批量推送
	enableBatch := true

	size := sendRepeat * int(time.Second/sendInterval) * 10

	var clients []*client
	for i := 0; i < connTotal; i++ {
		c := &client{}
		c.subList = []string{strconv.Itoa(i)}

		var sendList []string
		for j := 0; j < sendRepeat; j++ {
			sendList = append(sendList, strconv.Itoa(i))
		}
		c.sendList = sendList
		c.addr = addr
		c.size = size
		clients = append(clients, c)
	}

	wg := &sync.WaitGroup{}
	wg.Add(len(clients))
	fmt.Printf("连接总数 %v\n", len(clients))
	for _, client := range clients {
		client.creatConn()
		client.runConn()
		go client.doSub(wg)
	}
	wg.Wait()
	time.Sleep(time.Second)

	go refreshTerminal(flushInterval, clients, aggregationInterval)

	for _, client := range clients {
		go client.doSend(sendInterval, enableBatch)
	}

	for {
		s, _, _ := bufio.NewReader(os.Stdin).ReadLine()
		if string(s) == "q" {
			return
		}
	}
}

func refreshTerminal(flushInterval time.Duration, clients []*client, aggregationInterval time.Duration) {
	var conns []*core.Conn

	for _, c := range clients {
		conns = append(conns, c.c)
	}

	roomTerminalReport := core.NewReporter(conns, aggregationInterval)
	roomTerminalReport.Run()
	tick := time.Tick(flushInterval)
	for {
		<-tick
		roomTerminalReport.ShowToTerminal()
	}
}
