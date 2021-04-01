package routego

import (
	"bytes"
	"context"
	"errors"
	"github.com/gorilla/websocket"
	"github.com/wiqun/route-go/message"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type Client interface {
	//批量发布消息
	//如果设置了message.PubRequest.NotFound字段,服务器会在找不到订阅者时会此字段原因返回,通常配合着SetOnNotFoundHandler一起使用
	BatchPublish(msg []*message.PubRequest) error

	//批量订阅
	BatchSubscribe(topicList []string) error

	//批量取消订阅
	BatchUnsubscribe(topicList []string) error

	//查询主题是否有订阅者
	//如果传入了customData,则返回的结果的会把customData原样返回
	Query(topicList []string, customData []byte) error

	//连接到服务器
	Connect() error

	//设置消息回调
	SetOnMessageHandler(onMessageHandler OnMessageHandler)

	//设置查询结果回调
	SetOnQueryResultHandler(onQueryResultHandler OnQueryResultHandler)

	//设置NotFound回调
	SetOnNotFoundHandler(onNotFoundHandler OnNotFoundHandler)
	io.Closer
}

type BatchTopicOpMessage struct {
	IsSub     bool
	TopicList []string
}

type OnMessageHandler func([]*message.Message)

type OnQueryResultHandler func(*message.QueryResponse)

type OnNotFoundHandler func(notFound []byte)

type client struct {
	topicOp              chan BatchTopicOpMessage
	pub                  chan []*message.PubRequest
	query                chan *message.QueryRequest
	options              ConnOption
	subMap               map[string]struct{}
	dialer               *websocket.Dialer
	log                  Logger
	onMessageHandler     OnMessageHandler
	onQueryResultHandler OnQueryResultHandler
	onNotFoundHandler    OnNotFoundHandler

	ic *innerConn

	once   uint32
	closed bool
	sync.Mutex
}

type ConnOption struct {
	TopicOpChanSize int
	PubChanSize     int
	QueryChanSize   int
	Url             string
	AutoReconnect   bool

	//如果在KeepAlive没有发送过消息则会发送一个心跳消息,一般为ReadTimeout/2
	KeepAlive time.Duration
	//与服务器连接的超时时长
	ConnectTimeout time.Duration

	//写入超时时长
	WriteTimeout time.Duration

	//读取超时时长
	ReadTimeout time.Duration

	//重连的最大延迟间隔
	MaxReconnectInterval time.Duration
}

func NewDefaultOption(Url string) *ConnOption {
	return &ConnOption{
		TopicOpChanSize: 2000,
		PubChanSize:     4000,
		QueryChanSize:   1000,
		Url:             Url,
		AutoReconnect:   true,

		KeepAlive:            time.Minute,
		ConnectTimeout:       15 * time.Second,
		WriteTimeout:         30 * time.Second,
		ReadTimeout:          2 * time.Minute,
		MaxReconnectInterval: 32 * time.Second,
	}
}

func NewClient(options *ConnOption, logger Logger) (Client, error) {

	if options.WriteTimeout == 0 {
		return nil, errors.New("WriteTimeout 不能为0")
	}
	if options.ConnectTimeout == 0 {
		return nil, errors.New("ConnectTimeout 不能为0")
	}

	if options.KeepAlive == 0 {
		return nil, errors.New("KeepAlive 不能为0")
	}
	if options.AutoReconnect && options.MaxReconnectInterval == 0 {
		return nil, errors.New("MaxReconnectInterval 不能为0")
	}

	if logger == nil {
		logger = NullLogger{}
	}

	c := &client{
		options:              *options,
		topicOp:              make(chan BatchTopicOpMessage, options.TopicOpChanSize),
		pub:                  make(chan []*message.PubRequest, options.PubChanSize),
		query:                make(chan *message.QueryRequest, options.QueryChanSize),
		log:                  logger,
		subMap:               make(map[string]struct{}),
		onMessageHandler:     func(messages []*message.Message) {},
		onQueryResultHandler: func(response *message.QueryResponse) {},
		onNotFoundHandler:    func(notFound []byte) {},
		dialer:               &websocket.Dialer{HandshakeTimeout: options.ConnectTimeout, Proxy: http.ProxyFromEnvironment},
	}

	return c, nil
}
func (c *client) Connect() error {
	return c.connect()
}

func (c *client) SetOnMessageHandler(onMessageHandler OnMessageHandler) {
	if onMessageHandler == nil {
		panic("onMessageHandler==nil")
	}
	c.onMessageHandler = onMessageHandler
}

func (c *client) SetOnQueryResultHandler(onQueryResultHandler OnQueryResultHandler) {
	if onQueryResultHandler == nil {
		panic("onQueryResultHandler==nil")
	}
	c.onQueryResultHandler = onQueryResultHandler
}

func (c *client) SetOnNotFoundHandler(onNotFoundHandler OnNotFoundHandler) {
	if onNotFoundHandler == nil {
		panic("onNotFoundHandler==nil")
	}
	c.onNotFoundHandler = onNotFoundHandler
}

var ClosedError = errors.New("连接已关闭")

func (c *client) BatchPublish(msg []*message.PubRequest) error {
	if c.closed {
		return ClosedError
	}
	c.pub <- msg
	return nil
}
func (c *client) BatchSubscribe(topicList []string) error {
	if c.closed {
		return ClosedError
	}
	c.topicOp <- BatchTopicOpMessage{IsSub: true, TopicList: topicList}
	return nil

}

func (c *client) BatchUnsubscribe(topicList []string) error {
	if c.closed {
		return ClosedError
	}
	c.topicOp <- BatchTopicOpMessage{IsSub: false, TopicList: topicList}
	return nil

}
func (c *client) Query(topicList []string, customData []byte) error {
	if c.closed {
		return ClosedError
	}
	c.query <- &message.QueryRequest{TopicList: topicList, CustomData: customData}
	return nil

}

func (c *client) connect() error {
	if c.ic != nil {
		return errors.New("重复调用connect")
	}
	socket, _, err := c.dialer.Dial(c.options.Url, nil)
	if err != nil {
		return err
	}
	c.ic = newInnerConn(c, socket, c.onClose)
	c.ic.run()
	return nil
}

func (c *client) Close() error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return ClosedError
	}
	c.closed = true
	c.ic.close()
	return nil
}

func (c *client) onClose() {
	if c.options.AutoReconnect {
		c.reconnect()
	} else {
		c.Close()
	}
}

func (c *client) reconnect() {
	if !atomic.CompareAndSwapUint32(&c.once, 0, 1) { //已有别的reconnect在跑
		return
	}
	defer atomic.StoreUint32(&c.once, 0)

	c.ic.close()

	wait := time.Second

	sleep := func() {
		time.Sleep(wait)
		wait = wait * 2
		if wait > c.options.MaxReconnectInterval {
			wait = c.options.MaxReconnectInterval
		}
	}
	for {
		if c.closed {
			return
		}

		closeSignal := make(chan struct{})
		go c.processSubUnsub(closeSignal)

		socket, _, err := c.dialer.Dial(c.options.Url, nil)

		closeSignal <- struct{}{}

		if err != nil {
			c.log.PrintlnError("重新拨号失败:", err)
			sleep()
			continue
		}

		err = c.resubscribe(socket)

		if err != nil {
			c.log.PrintlnError("重新订阅失败:", err)
			sleep()
			continue
		}

		c.Lock()
		if !c.closed {
			c.ic = newInnerConn(c, socket, c.onClose)
			c.ic.run()
			c.log.Println("重新连接成功")
		} else {
			socket.Close()
		}
		c.Unlock()
		return

	}

}

//在断线重连的过程中也处理Sub/UnSub请求,只存放在map里
func (c *client) processSubUnsub(done chan struct{}) {

	for {
		select {
		case <-done:
			return
		case op := <-c.topicOp:
			if op.IsSub {
				for _, topic := range op.TopicList {
					c.subMap[topic] = struct{}{}
				}

			} else {
				for _, topic := range op.TopicList {
					delete(c.subMap, topic)
				}
			}

		}
	}
}

//需要自行保证此方法并发写入安全
func (c *client) resubscribe(socket *websocket.Conn) error {
	if len(c.subMap) == 0 {
		return nil
	}

	batchSub := make([]string, 0, len(c.subMap))
	for topic, _ := range c.subMap {
		batchSub = append(batchSub, topic)
	}

	req := &message.ClientMessage{
		Type: message.ClientType_BatchSubType,
		BatchSub: &message.BatchSubRequest{
			TopicList: batchSub,
		},
	}
	c.log.Println("重新订阅主题")
	data, err := req.Marshal()
	if err != nil {
		return err
	}
	socket.SetWriteDeadline(time.Now().Add(c.options.WriteTimeout))
	err = socket.WriteMessage(websocket.BinaryMessage, data)
	if err != nil {
		return err
	}
	return nil
}

type onClose func()

type innerConn struct {
	socket  *websocket.Conn
	c       *client
	readBuf bytes.Buffer

	wg         *sync.WaitGroup
	context    context.Context
	cancelFunc context.CancelFunc

	onClose onClose

	sync.Once
}

func newInnerConn(c *client, socket *websocket.Conn, onClose onClose) *innerConn {
	ctx, cancelFunc := context.WithCancel(context.Background())
	return &innerConn{
		socket:     socket,
		c:          c,
		wg:         &sync.WaitGroup{},
		context:    ctx,
		cancelFunc: cancelFunc,
		onClose:    onClose,
	}
}

func (ic *innerConn) run() {
	ic.wg.Add(2)
	go ic.read()
	go ic.send()
}

func (ic *innerConn) readFrom(r io.Reader) (data []byte, err error) {
	ic.readBuf.Reset()
	// If the buffer overflows, we will get bytes.ErrTooLarge.
	// Return that as an error. Any other panic remains.
	defer func() {
		e := recover()
		if e == nil {
			return
		}
		if panicErr, ok := e.(error); ok && panicErr == bytes.ErrTooLarge {
			err = panicErr
		} else {
			panic(e)
		}
	}()
	_, err = ic.readBuf.ReadFrom(r)
	return ic.readBuf.Bytes(), err
}

func (ic *innerConn) read() {
	defer ic.wg.Done()
	ic.socket.SetReadDeadline(time.Now().Add(ic.c.options.ReadTimeout))
	ic.socket.SetPongHandler(func(string) error { ic.socket.SetReadDeadline(time.Now().Add(ic.c.options.ReadTimeout)); return nil })
	for {
		t, r, err := ic.socket.NextReader()

		if err != nil {
			ic.c.log.PrintlnError("NextReader错误:", err)
			go ic.close()
			return
		}

		data, err := ic.readFrom(r)

		if err != nil {
			ic.c.log.PrintlnError("readFrom错误:", err)
			go ic.close()
			return
		}

		if t != websocket.BinaryMessage {
			continue
		}

		resp := message.ServerMessage{}
		err = resp.Unmarshal(data)
		if err != nil {
			ic.c.log.PrintlnError("ResponseMessage Unmarshal 错误:", err)
			continue
		}

		switch resp.Type {
		case message.ServerType_MessagesType:
			if resp.Messages != nil {
				ic.c.onMessageHandler(resp.Messages)
			}
		case message.ServerType_QueryResponseType:
			if resp.Query != nil {
				ic.c.onQueryResultHandler(resp.Query)
			}
		case message.ServerType_PubNotFoundType:
			if resp.PubNotFound != nil {
				ic.c.onNotFoundHandler(resp.PubNotFound)
			}
		}
	}

}

func (ic *innerConn) send() {
	defer ic.wg.Done()

	var req message.ClientMessage

	ticker := time.NewTicker(ic.c.options.KeepAlive)
	defer ticker.Stop()

	var buf []byte
	for {
		select {
		case <-ic.context.Done():
			return
		case <-ticker.C:
			//todo 优化如果最近发送过别的消息是否可以取消发送心跳包?
			err := ic.socket.WriteControl(websocket.PingMessage, nil, time.Now().Add(ic.c.options.WriteTimeout))
			if err != nil {
				ic.c.log.PrintlnError("发送心跳错误:", err)
				go ic.close()
				return
			}
			continue
		case op := <-ic.c.topicOp:
			if op.IsSub {
				for _, topic := range op.TopicList {
					ic.c.subMap[topic] = struct{}{}
				}
				req = message.ClientMessage{
					Type:     message.ClientType_BatchSubType,
					BatchSub: &message.BatchSubRequest{TopicList: op.TopicList},
				}
			} else {
				for _, topic := range op.TopicList {
					delete(ic.c.subMap, topic)
				}
				req = message.ClientMessage{
					Type:       message.ClientType_BatchUnSubType,
					BatchUnsub: &message.BatchUnSubRequest{TopicList: op.TopicList},
				}
			}
		case pub := <-ic.c.pub:
			if len(pub) == 0 {
				continue
			}
			req = message.ClientMessage{
				Type:     message.ClientType_BatchPubType,
				BatchPub: &message.BatchPubRequest{Batch: pub},
			}
		case query := <-ic.c.query:
			if len(query.TopicList) == 0 {
				continue
			}
			req = message.ClientMessage{
				Type:  message.ClientType_QueryType,
				Query: query,
			}
		}

		size := req.Size()
		if cap(buf) < size {
			buf = make([]byte, 0, size)
		}
		buf = buf[:size]

		_, err := req.MarshalTo(buf)
		if err != nil {
			ic.c.log.PrintlnError("RequestMessage  序列化失败:", err)
			continue
		}

		ic.socket.SetWriteDeadline(time.Now().Add(ic.c.options.WriteTimeout))
		err = ic.socket.WriteMessage(websocket.BinaryMessage, buf)

		if err != nil {
			ic.c.log.PrintlnError("写入到socket错误:", err)
			go ic.close()
			return
		}

	}
}

func (ic *innerConn) close() error {
	err := ClosedError
	ic.Do(func() {
		err = nil
		ic.cancelFunc()

		//错误如何处理
		err = ic.socket.Close()
		ic.wg.Wait()
		go ic.onClose() //使用go关键词, 如果不使用onClose再又再次调用ic.close将会发生死锁
	})

	return err
}
