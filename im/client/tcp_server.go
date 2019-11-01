package main

import (
	"im2/common"
	"net"
	"sync"
	"time"

	"github.com/lunny/log"
)

type Server struct {
	sync.Mutex
	csServer     *common.Conn
	exitChan     chan struct{}
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func newServer() *Server {
	return &Server{
		exitChan: make(chan struct{}),
	}
}

func (s *Server) regCs(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		log.Infof("dial %s fail, err:%s", addr, err)
	}

	authMsg := common.NewMessage()
	header := map[string]interface{}{
		"auth": "im client uid",
		"uid":  fromUid,
	}
	authMsg.Head = header
	data, err := authMsg.Encode()
	if err != nil {
		log.Printf("shake hand fail, err:%s", err)
		return err
	}
	_, err = conn.Write(data)
	if err != nil {
		log.Printf("shake hand fail, err:%s", err)
		return err
	}

	c := common.NewConn(conn)
	s.csServer = c

	go s.serve(c)

	return nil
}

func (s *Server) serve(c *common.Conn) {
	defer func() {
		c.Close()
	}()

	go c.Write()

	for {
		select {
		case <-s.exitChan:
			return
		default:
		}

		t0 := time.Now()
		if s.readTimeout != 0 {
			c.Conn.SetReadDeadline(t0.Add(s.readTimeout))
		}

		msg := common.NewMessage()
		err := msg.Decode(c.Conn)
		if err != nil {
			log.Println("err:", err)
			break
		}

		// 收到cs 的消息
		log.Infof("im client receive msg body:%s", msg.Body)
	}
}

func (s *Server) stop() {
	select {
	case <-s.exitChan:
	default:
		close(s.exitChan)
	}

	s.csServer.Close()
}
