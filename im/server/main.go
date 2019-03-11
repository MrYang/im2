package main

import (
	"os/signal"
	"syscall"
	"os"
	"github.com/lunny/log"
	"flag"
	"zz.com/im2/common"
	"time"
)

var serverId int
var redisOp *common.RedisOp

func main() {
	si := flag.Int("serverId", 1, "serverId")
	redisAddr := flag.String("redis-addr", "redis://127.0.0.1:6379", "redisAddr")

	flag.Parse()

	serverId = *si
	redisOp = common.NewRedisOp(*redisAddr)

	server := newServer()
	server.regConsul()

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				server.regConsul()
			case <-server.exitChan:
				return
			}
		}
	}()

	sg := make(chan os.Signal)
	signal.Notify(sg, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGKILL)

	select {
	case s := <-sg:
		log.Println("got signal", s)
	}

	stop := make(chan struct{})
	go func() {
		server.stop()
		stop <- struct{}{}
	}()

	<-stop

	log.Println("im server is stop")
}
