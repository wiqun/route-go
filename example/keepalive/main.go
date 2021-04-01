package main

import (
	"fmt"
	RouteGo "github.com/wiqun/route-go"
	"os"
	"os/signal"
	"syscall"
	"time"
)

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

func main() {

	options := RouteGo.NewDefaultOption("ws://127.0.0.1:4000")
	options.KeepAlive = 2 * time.Second
	options.ReadTimeout = 6 * time.Second
	c, err := RouteGo.NewClient(options, FmtLogger{})

	if err != nil {
		panic(err)
	}

	err = c.Connect()

	if err != nil {
		panic(err)
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-signalChan: //wait exit signal from Docker
			time.Sleep(2 * time.Second)
			c.Close()
			return
		}
	}

}
