package core

import "time"

type Sender struct {
	c            *Conn
	sendTopics   []string
	sendInterval time.Duration
}

func NewSender(c *Conn, sendTopics []string, sendInterval time.Duration) *Sender {
	return &Sender{c: c, sendInterval: sendInterval, sendTopics: sendTopics}
}

func (s *Sender) Run(enableBatch bool) {
	go func() {
		ticker := time.NewTicker(s.sendInterval)
		for {
			select {
			case <-ticker.C:
				if enableBatch {
					s.c.Send(s.sendTopics)
				} else {
					for _, topic := range s.sendTopics {
						s.c.Send([]string{topic})
					}
				}
				break
			}
		}
	}()
}
