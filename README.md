### 简介
一个Route客户端,可以与[Route](https://github.com/wiqun/route)服务器进行交互
1. 支持自动重连
2. 订阅的主题重连后自动重新订阅
3. 高性能

### 快速开始
```shell
$ go get -u github.com/wiqun/route-go
```


```go
import . "github.com/wiqun/route-go"
func main() {
	c, err := NewClient(NewDefaultOption("ws://127.0.0.1:4000"), nil)
	if err != nil {
		panic(err)
	}

	//消息回调
	c.SetOnMessageHandler(func(messages []*message.Message) {
		for _, m := range messages {
			fmt.Println(string(m.Payload))
		}
	})

	//连接
	err = c.Connect()
	if err != nil {
		panic(err)
	}

	//订阅主题
	err = c.BatchSubscribe([]string{"hello"})
	if err != nil {
		panic(err)
	}
	time.Sleep(time.Second*2)


	//发布消息
	c.BatchPublish([]*message.PubRequest{{Topic: "hello", Payload: []byte("hello world")}})

	time.Sleep(time.Second*10)
}
//hello world
```
更多例子请参看[example](example)目录