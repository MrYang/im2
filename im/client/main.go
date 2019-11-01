package main

import (
	"bufio"
	"flag"
	"fmt"
	"im2/common"
	"os"
	"os/signal"
	"syscall"

	"github.com/lunny/log"
)

var fromUid, toUid uint64

func main() {

	uid := flag.Uint64("uid", 1, "from uid")
	tuid := flag.Uint64("toUid", 2, "to uid")
	csPort := flag.Int("csPort", 3004, "cs port")

	flag.Parse()

	fromUid = *uid
	toUid = *tuid

	server := newServer()
	server.regCs(fmt.Sprintf("127.0.0.1:%d", *csPort))

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			text := scanner.Text()

			msg := common.NewMessage()
			header := map[string]interface{}{
				"uid":        toUid,
				"serverType": "im",
			}
			msg.Head = header
			msg.Body = []byte(text)

			server.csServer.WriteChan <- msg
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

	log.Println("im client is stop")
}
