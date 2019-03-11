package main

import (
	"time"
	"sync"
	"net"
	"github.com/hashicorp/consul/api"
	"zz.com/im2/common"
	"github.com/lunny/log"
	"strings"
	"fmt"
	"github.com/spf13/cast"
)

type Server struct {
	sync.Mutex
	csServers    map[int]*common.Conn
	exitChan     chan struct{}
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func newServer() *Server {
	return &Server{
		csServers: make(map[int]*common.Conn),
		exitChan:  make(chan struct{}),
	}
}

func (s *Server) regConsul() {
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		log.Fatal(err)
	}

	services, err := client.Agent().Services()

	for _, v := range services {
		for _, t := range v.Tags {
			if t == "cs" {
				arr := strings.Split(v.ID, "_")
				id := cast.ToInt(arr[1])
				if _, ok := s.csServers[id]; !ok {
					s.regCs(fmt.Sprintf("%s:%d", v.Address, v.Port), id)
				}
				break
			}
		}
	}
}

func (s *Server) regCs(addr string, id int) error {
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		log.Infof("dial %s fail, err:%s", addr, err)
	}

	shakeHandMsg := common.NewMessage()
	header := map[string]interface{}{
		"serverType": "im",
		"serverId":   serverId,
	}
	shakeHandMsg.Head = header
	data, err := shakeHandMsg.Encode()
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
	c.Data["csId"] = id
	s.csServers[id] = c

	go s.serve(c)

	return nil
}

func (s *Server) serve(c *common.Conn) {
	defer func() {
		s.delConn(c)
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

		// 收到cs 的消息, 转发给to uid 的cs
		log.Infof("msg body:%s", msg.Body)

		if msg.IsHeartbeat() {
			continue
		}
		uid, ok := msg.Head["uid"]
		if !ok {
			continue
		}

		uid64 := cast.ToUint64(uid)

		csId, err := redisOp.GetCsId(uid64)
		if err != nil {
			continue
		}

		if csConn, ok := s.csServers[csId]; ok {
			csConn.WriteChan <- msg
		}
	}
}

func (s *Server) delConn(conn *common.Conn) {
	s.Lock()
	defer s.Unlock()

	if serverId, ok := conn.Data["csId"]; ok {
		delete(s.csServers, serverId.(int))
	}
}

func (s *Server) stop() {
	select {
	case <-s.exitChan:
	default:
		close(s.exitChan)
	}

	for _, c := range s.csServers {
		c.Close()
	}
}
