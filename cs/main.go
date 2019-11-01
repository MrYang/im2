package main

import (
	"flag"
	"fmt"
	"im2/common"
	"log"
	"os"
	"os/signal"
	"syscall"
)

var serverPort, clientPort, httpPort, serverId int

func main() {

	sp := flag.Int("serverPort", 3003, "serverPort")
	cp := flag.Int("clientPort", 3004, "clientPort")
	hp := flag.Int("httpPort", 3005, "httpPort")
	si := flag.Int("serverId", 1, "serverId")
	redisAddr := flag.String("redis-addr", "redis://127.0.0.1:6379", "redisAddr")

	flag.Parse()

	serverPort = *sp
	clientPort = *cp
	httpPort = *hp
	serverId = *si
	redisOp = common.NewRedisOp(*redisAddr)

	backendServer := newBackendServer()
	clientServer := newClientServer()
	httpServer := &httpServer{}

	singleBackendServer = backendServer
	singleClientServer = clientServer

	serviceId := fmt.Sprintf("cs_%d", serverId)
	backendServer.regConsul(serviceId, serverPort)
	go backendServer.start()
	go clientServer.start()
	go httpServer.Start()

	sg := make(chan os.Signal)
	signal.Notify(sg, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGKILL)

	select {
	case s := <-sg:
		log.Println("got signal", s)
	}

	stop := make(chan struct{})
	go func() {
		backendServer.unRegConsul(serviceId)
		clientServer.stop()
		backendServer.stop()
		stop <- struct{}{}
	}()

	<-stop

	log.Println("cs is stop")
}
