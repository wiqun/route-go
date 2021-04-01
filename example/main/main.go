package main

import (
	"bufio"
	"fmt"
	cli "github.com/jawher/mow.cli"
	RouteGo "github.com/wiqun/route-go"
	"github.com/wiqun/route-go/message"
	"log"
	"os"
)

func main() {
	app := cli.App("clientdemo", "一个简单的命令行client")
	addr := app.StringOpt("a addr", "127.0.0.1:4000", "route地址")
	topicList := app.StringsOpt("t topic", []string{"pressure_test"}, "订阅的主题")
	content := app.StringOpt("c content", "testContent", "订阅的主题")
	app.Action = func() {
		run("ws://"+*addr, *content, *topicList)
	}

	app.Run(os.Args)
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

func run(addr string, content string, topicList []string) {
	chanRecv := make(chan string, 1000)  //同步显示用的管道
	chanSend := make(chan string, 1000)  //同步显示用的管道
	chanError := make(chan string, 1000) //同步显示用的管道
	readCount := 0
	c, err := RouteGo.NewClient(RouteGo.NewDefaultOption(addr), FmtLogger{})

	c.SetOnMessageHandler(func(messages []*message.Message) {
		readCount++
		chanRecv <- fmt.Sprintf("recv(%d) : %+v",
			readCount, messages)
	})

	c.SetOnQueryResultHandler(func(response *message.QueryResponse) {
		readCount++
		chanRecv <- fmt.Sprintf("recv(%d) : %+v",
			readCount, response)
	})
	c.SetOnNotFoundHandler(func(notfound []byte) {
		readCount++
		chanRecv <- fmt.Sprintf("recv(%d) : %+v",
			readCount, string(notfound))
	})

	if err != nil {
		log.Fatal("dial:", err)
	}

	err = c.Connect()

	if err != nil {
		log.Fatal("Connect:", err)
	}

	defer c.Close()

	fmt.Println("press 'q' 退出, 's' 订阅, 'u' 取消订阅, 'p' 发布, 'n' 发布到不存在的主题, 'r' 查询 ,'c'关闭连接 ")

	go func() { //同步显示
		for {
			select {
			case r := <-chanRecv:
				fmt.Println(r)
				break
			case send := <-chanSend:
				fmt.Println(send)
				break
			case err := <-chanError:
				fmt.Println(err)
				break
			}
		}
	}()

	//var count int = 0

	for {
		reader := bufio.NewReader(os.Stdin)
		char, _, err := reader.ReadRune()
		if err != nil {
			fmt.Println(err)
		}

		var msg string
		switch char {
		case 'q':
			return
		case 's':
			e := c.BatchSubscribe(topicList)
			if e != nil {
				chanError <- fmt.Sprintf("%v", e)
				continue
			}
			msg = fmt.Sprintf("subMsg : %+v", topicList)
		case 'u':
			e := c.BatchUnsubscribe(topicList)
			if e != nil {
				chanError <- fmt.Sprintf("%v", e)
				continue
			}
			msg = fmt.Sprintf("unsubMsg : %+v", topicList)

		case 'p':
			var pubList []*message.PubRequest
			for _, topic := range topicList {
				pubList = append(pubList, &message.PubRequest{
					Topic:   topic,
					Payload: []byte(content),
				})
			}

			e := c.BatchPublish(pubList)
			if e != nil {
				chanError <- fmt.Sprintf("%v", e)
				continue
			}

			msg = fmt.Sprintf("pubMsg : %+v", pubList)
		case 'n':
			var pubList []*message.PubRequest
			pubList = append(pubList, &message.PubRequest{
				Topic:    "topictopictopictopic",
				Payload:  []byte(content),
				NotFound: []byte("notfound"),
			})

			e := c.BatchPublish(pubList)
			if e != nil {
				chanError <- fmt.Sprintf("%v", e)
				continue
			}

			msg = fmt.Sprintf("pubMsg : %+v", pubList)

		case 'r':
			e := c.Query(topicList, nil)
			if e != nil {
				chanError <- fmt.Sprintf("%v", e)
				continue
			}
			msg = fmt.Sprintf("query : %+v", topicList)

		case 'c':
			c.Close()

		default:
			continue
		}
		fmt.Println(msg)
	}
}
