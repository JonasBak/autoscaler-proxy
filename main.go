package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/JonasBak/autoscaler-proxy/proxy"
	"github.com/JonasBak/autoscaler-proxy/utils"
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
	if config.Autoscaler.HCloudToken == "" {
		config.Autoscaler.HCloudToken = os.Getenv("HCLOUD_TOKEN")
	}

	p := proxy.New(config)

	fatal := make(chan struct{}, 1)

	ctx, cancel := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, "fatal", fatal)

	go p.Start(ctx)

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGTERM)
		<-c
		log.Warn("Killing...")
		p.Kill()
		log.Warn("Killed")
		os.Exit(0)
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	select {
	case <-fatal:
		log.Warn("Fatal error received, shutting down...")
		cancel()
		p.Stop()
		log.Debug("Stopped")
		os.Exit(1)
		break
	case <-c:
		log.Warn("Shutting down...")
		cancel()
		break
	}
	p.Stop()
	log.Debug("Stopped")
}
