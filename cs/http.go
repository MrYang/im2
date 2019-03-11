package main

import (
	"github.com/spf13/cast"
	"net/http"
	"github.com/lunny/log"
	"fmt"
	"github.com/gin-gonic/gin"
)

type httpServer struct {
}

func (httpServer *httpServer) Start() {

	addr := fmt.Sprintf("127.0.0.1:%d", httpPort)
	log.Info("http listen:", addr)

	gin.SetMode(gin.ReleaseMode)
	route := gin.Default()
	route.GET("/ping", func(context *gin.Context) {
		context.JSON(http.StatusOK, "ok")
	})

	route.GET("/servers", func(context *gin.Context) {
		context.JSON(http.StatusOK, singleBackendServer.serverTag)
	})

	route.GET("/clients", func(context *gin.Context) {
		uid := context.Param("uid")
		iuid := cast.ToUint64(uid)
		if a, ok := singleClientServer.conns[iuid]; ok {
			context.JSON(http.StatusOK, a.ID())
		} else {
			context.JSON(http.StatusOK, "not online")
		}
	})

	route.Run(addr)
}
