package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/JonasBak/docker-api-autoscaler-proxy/proxy"
	"github.com/JonasBak/docker-api-autoscaler-proxy/utils"
)

var log = utils.Logger().WithField("pkg", "main")

func main() {
	p := proxy.New("127.0.0.1:8081")

	ctx, cancel := context.WithCancel(context.Background())

	go p.Start(ctx)

	c := make(chan os.Signal, 1)

	signal.Notify(c, os.Interrupt) //, os.Kill)

	select {
	case <-c:
		log.Warn("Shutting down...")
		cancel()
		break
	}
	p.Stop()
}
