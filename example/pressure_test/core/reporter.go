package core

import (
	"fmt"
	"strings"
	"time"
)

type Reporter struct {
	conns               []*Conn
	aggregationInterval time.Duration

	//发送速度
	sendSpeed float64

	//接收速度
	recvSpeed float64

	//平均接收速度
	recvSpeedAvg float64
}

func NewReporter(conns []*Conn, aggregationInterval time.Duration) *Reporter {
	return &Reporter{
		conns:               conns,
		aggregationInterval: aggregationInterval,
	}
}

func (r *Reporter) Run() {
	go func() {
		timer := time.NewTicker(r.aggregationInterval)

		var previousSendCount int64
		var previousRecvTotal int64
		var previousRecvCount int64

		for {
			select {

			case <-timer.C:
				var sendCount int64
				var recvTotal int64
				var recvCount int64

				var sendCountDiff int64
				var recvTotalDiff int64
				var recvCountDiff int64
				for _, client := range r.conns {
					sendCount += client.SendCount
					recvTotal += client.RecvTotal
					recvCount += client.RecvCount
				}

				sendCountDiff += sendCount - previousSendCount
				recvTotalDiff += recvTotal - previousRecvTotal
				recvCountDiff += recvCount - previousRecvCount

				previousSendCount += sendCountDiff
				previousRecvTotal += recvTotalDiff
				previousRecvCount += recvCountDiff

				d := r.aggregationInterval.Seconds()
				r.sendSpeed = float64(sendCountDiff) / d
				r.recvSpeed = float64(recvCountDiff) / d

				if recvCountDiff != 0 {
					r.recvSpeedAvg = float64(recvTotalDiff / recvCountDiff)
				}
			}
		}
	}()

}

func row(items []string) string {
	return strings.Join(items, "		")
}

func (r *Reporter) ShowToTerminal() {
	var column []string
	column = append(column, row([]string{
		"----------------------------------------user报告----------------------------------------",
	}))
	column = append(column, row([]string{
		fmt.Sprintf("发送消息速度: %.2f 条/秒", r.sendSpeed),
	}))

	column = append(column, row([]string{
		fmt.Sprintf("接收消息速度: %.2f 条/秒", r.recvSpeed),
		fmt.Sprintf("每条平均接收消息耗时: %.2f ms/条", r.recvSpeedAvg),
	}))

	fmt.Println(strings.Join(column, "\n"))
}
