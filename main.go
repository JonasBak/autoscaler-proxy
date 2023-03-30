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
	config := DefaultConfig()
	if len(os.Args) > 1 {
		c, err := ParseConfigFile(os.Args[len(os.Args)-1])
		if err != nil {
			log.WithError(err).Fatal("Failed to parse config file")
		}
		config = c
	}
	p := proxy.New(config)

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
	log.Debug("Stopped")
}
