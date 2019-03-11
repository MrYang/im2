package main

import (
	"errors"
	"time"
	"sync"
	"net"
	"github.com/hashicorp/consul/api"
	"zz.com/im2/common"
	"github.com/lunny/log"
	"fmt"
	"github.com/spf13/cast"
)

var serverTypeErr = errors.New("server type error")

var singleBackendServer *BackendServer

var singleClientServer *ClientServer

var redisOp *common.RedisOp

type BackendServer struct {
	sync.Mutex
	serverTag    map[string]map[int32]*common.Conn
	exitChan     chan struct{}
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func newBackendServer() *BackendServer {
	return &BackendServer{
		serverTag: make(map[string]map[int32]*common.Conn),
		exitChan:  make(chan struct{}),
	}
}

func (server *BackendServer) regConsul(ID string, port int) {
	service := api.AgentServiceRegistration{
		ID:      ID,
		Name:    ID,
		Port:    port,
		Address: "127.0.0.1",
		Tags:    []string{"cs"},
	}

	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		log.Fatal(err)
	}

	if err = client.Agent().ServiceRegister(&service); err != nil {
		log.Fatal(err)
	}
}

func (server *BackendServer) unRegConsul(ID string) error {
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		log.Fatal(err)
	}

	return client.Agent().ServiceDeregister(ID)
}

func (server *BackendServer) start() {
	addr := fmt.Sprintf(":%d", serverPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	log.Info("cs listen", addr)

	defer func() {
		listener.Close()
	}()

	var tempDelay time.Duration

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-server.exitChan:
				break
			default:
			}

			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}

				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}

				log.Errorf("Accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			conn.Close()
			break
		}

		c, err := server.addConn(conn)
		if err != nil {
			conn.Close()
			break
		}

		go server.serve(c)
	}
}

func (server *BackendServer) serve(c *common.Conn) {
	defer func() {
		server.delConn(c)
		c.Close()
	}()

	go c.Write()

	for {
		select {
		case <-server.exitChan:
			return
		default:
		}

		t0 := time.Now()
		if server.readTimeout != 0 {
			c.Conn.SetReadDeadline(t0.Add(server.readTimeout))
		}

		msg := common.NewMessage()
		err := msg.Decode(c.Conn)
		if err != nil {
			log.Println("err:", err)
			break
		}

		if msg.IsHeartbeat() {
			continue
		}

		if uid, ok := msg.Head["uid"]; ok {
			uid64 := cast.ToUint64(uid)
			// 转发到client
			if clientConn, ok := singleClientServer.conns[uid64]; ok {
				if !clientConn.IsClose() {
					clientConn.WriteChan <- msg
				}
			}
		}
	}
}

func (server *BackendServer) addConn(conn net.Conn) (*common.Conn, error) {
	server.Lock()
	defer server.Unlock()

	c := common.NewConn(conn)
	msg := common.NewMessage()
	err := msg.Decode(conn)
	if err != nil {
		return nil, err
	}

	serverType, ok := msg.Head["serverType"]
	if !ok {
		return nil, serverTypeErr
	}

	serverId, ok := msg.Head["serverId"]
	if !ok {
		return nil, serverTypeErr
	}

	sId := cast.ToInt32(serverId)

	conns, ok := server.serverTag[serverType.(string)]
	if ok {
		conns[sId] = c
	} else {
		conns = make(map[int32]*common.Conn)
		conns[sId] = c
		server.serverTag[serverType.(string)] = conns
	}

	c.Data["serverType"] = msg.Head["serverType"]
	c.Data["serverId"] = sId

	c.WriteChan <- common.NewMessage()

	return c, nil
}

func (server *BackendServer) delConn(conn *common.Conn) {
	server.Lock()
	defer server.Unlock()

	if serverType, ok := conn.Data["serverType"]; ok {
		if conns, ok := server.serverTag[serverType.(string)]; ok {
			serverId := conn.Data["serverId"]
			delete(conns, serverId.(int32))
		}
	}
}

func (server *BackendServer) stop() {
	select {
	case <-server.exitChan:
	default:
		close(server.exitChan)
	}

	for _, t := range server.serverTag {
		for _, c := range t {
			c.Close()
		}
	}
}

type ClientServer struct {
	sync.Mutex
	conns        map[uint64]*common.Conn
	groups       map[string][]uint64
	count        uint32
	exitChan     chan struct{}
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func newClientServer() *ClientServer {
	return &ClientServer{
		conns:    make(map[uint64]*common.Conn),
		groups:   make(map[string][]uint64),
		exitChan: make(chan struct{}),
	}
}

func (server *ClientServer) start() {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", clientPort))
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		listener.Close()
	}()

	var tempDelay time.Duration

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-server.exitChan:
				break
			default:
			}

			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}

				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}

				log.Errorf("Accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			break
		}

		c, err := server.addConn(conn)
		if err != nil {
			conn.Close()
			break
		}

		go server.serve(c)
	}
}

func (server *ClientServer) serve(c *common.Conn) {
	defer func() {
		server.delConn(c)
		c.Close()
	}()

	go c.Write()

	for {

		select {
		case <-server.exitChan:
			return
		default:
		}

		t0 := time.Now()
		if server.readTimeout != 0 {
			c.Conn.SetReadDeadline(t0.Add(server.readTimeout))
		}

		msg := common.NewMessage()
		err := msg.Decode(c.Conn)
		if err != nil {
			log.Println("err:", err)
			break
		}

		log.Infof("%v", msg)
		if msg.IsHeartbeat() {
			continue
		}

		if serverType, ok := msg.Head["serverType"]; ok {
			// 转发到server
			if conns, ok := singleBackendServer.serverTag[serverType.(string)]; ok {
				for _, conn := range conns {
					log.Info("select backend server", conn.Data)
					if !conn.IsClose() {
						conn.WriteChan <- msg
						break
					}
				}
			}
		}
	}
}

func (server *ClientServer) addConn(conn net.Conn) (*common.Conn, error) {
	server.Lock()
	defer server.Unlock()

	c := common.NewConn(conn)
	msg := common.NewMessage()
	err := msg.Decode(conn)
	if err != nil {
		return nil, err
	}

	auth, ok := msg.Head["auth"]
	if !ok {
		return nil, serverTypeErr
	}

	log.Info("client auth:", auth)

	uid, ok := msg.Head["uid"]
	if !ok {
		return nil, serverTypeErr
	}

	if groups, ok := msg.Head["groups"]; ok {
		log.Info("groups:", groups)
	}


	uid64 := cast.ToUint64(uid)

	server.conns[uid64] = c
	c.Data["uid"] = uid64

	redisOp.SetCsId(uid64, serverId)

	return c, nil
}

func (server *ClientServer) delConn(conn *common.Conn) {
	server.Lock()
	defer server.Unlock()

	if uid, ok := conn.Data["uid"]; ok {
		uid64 := cast.ToUint64(uid)
		delete(server.conns, uid64)
		redisOp.DelCsId(uid64)
	}
}

func (server *ClientServer) stop() {
	select {
	case <-server.exitChan:
	default:
		close(server.exitChan)
	}

	for _, c := range server.conns {
		c.Close()
	}
}
