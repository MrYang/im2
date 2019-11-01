package common

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/lunny/log"
)

const (
	_ uint32 = iota
	Start
	Working
	Closed
)

type Conn struct {
	Conn      net.Conn
	WriteChan chan *Message
	ExitChan  chan struct{}
	Status    uint32
	Data      map[string]interface{}
}

func NewConn(conn net.Conn) *Conn {
	return &Conn{
		Conn:      conn,
		WriteChan: make(chan *Message, 16),
		ExitChan:  make(chan struct{}),
		Status:    Start,
		Data:      make(map[string]interface{}),
	}
}

func (c *Conn) String() string {
	return fmt.Sprintf("Remote=%s", c.Conn.RemoteAddr().String())
}

func (c *Conn) ID() string {
	return c.Conn.RemoteAddr().String()
}

func (c *Conn) Close() error {
	c.Status = Closed

	select {
	case <-c.ExitChan:
	default:
		// 所有<-c.ExitChan 都接收到
		close(c.ExitChan)
	}

	return c.Conn.Close()
}

func (c *Conn) IsClose() bool {
	return c.Status == Closed
}

func (c *Conn) Write() {
	defer func() {
		c.Close()
		close(c.WriteChan)
	}()
	for {
		select {
		case msg, ok := <-c.WriteChan:
			if !ok {
				return
			}
			if msg == nil {
				continue
			}

			data, err := msg.Encode()
			if err != nil {
				return
			}

			if _, err := c.Conn.Write(data); err != nil {
				log.Println("error:", err)
				return
			}
		case <-c.ExitChan:
			return
		}
	}
}

func (c *Conn) MarshalJSON() ([]byte, error) {
	co := struct {
		RemoteAddr string
		Status     uint32
		Data       map[string]interface{}
	}{
		RemoteAddr: c.ID(),
		Status:     c.Status,
		Data:       c.Data,
	}

	return json.Marshal(co)
}
