package routego

import (
	"github.com/stretchr/testify/assert"
	"github.com/wiqun/route-go/message"
	"testing"
	"time"
)

func TestConnect(t *testing.T) {
	clientA(t)
	time.Sleep(1 * time.Second)
	clientB(t)

	// stop after 10 seconds
	time.Sleep(1 * time.Second)
}

type FmtLogger struct {
	t *testing.T
}

func (f FmtLogger) Printf(format string, args ...interface{}) {
	f.t.Logf(format, args...)
}

func (f FmtLogger) Println(v ...interface{}) {
	f.t.Log(v...)
}

func (f FmtLogger) PrintlnError(v ...interface{}) {
	f.t.Error(v...)
}

func (f FmtLogger) PrintfError(format string, v ...interface{}) {
	f.t.Errorf(format, v...)
}

func clientA(t *testing.T) {
	c, err := NewClient(NewDefaultOption("ws://192.168.1.38:4002"), FmtLogger{
		t: t,
	})

	if err != nil {
		panic(err)
	}

	c.SetOnMessageHandler(func(messages []*message.Message) {

		assert.Len(t, messages, 3)
		assert.ElementsMatch(t, messages, []*message.Message{{Topic: "test1", Payload: []byte("data1")},
			{Topic: "test2", Payload: []byte("data2")},
			{Topic: "test3", Payload: []byte("data3")}})

	})

	err = c.Connect()
	if err != nil {
		panic(err)
	}

	err = c.BatchSubscribe([]string{"test1", "test2", "test3"})
	if err != nil {
		panic(err)
	}

}

func clientB(t *testing.T) {
	c, err := NewClient(NewDefaultOption("ws://192.168.1.38:4002"), FmtLogger{
		t: t,
	})

	if err != nil {
		panic(err)
	}

	c.SetOnQueryResultHandler(func(response *message.QueryResponse) {
		assert.ElementsMatch(t, response.TopicList, []string{"test1", "test2", "test3"})
		assert.EqualValues(t, response.CustomData, []byte("this custom data"))
	})

	err = c.Connect()
	if err != nil {
		panic(err)
	}

	err = c.Query([]string{"test1", "test2", "test3", "test4"}, []byte("this custom data"))
	if err != nil {
		panic(err)
	}

	err = c.BatchPublish([]*message.PubRequest{{Topic: "test1", Payload: []byte("data1")},
		{Topic: "test2", Payload: []byte("data2")},
		{Topic: "test3", Payload: []byte("data3")}})
	if err != nil {
		panic(err)
	}

}
