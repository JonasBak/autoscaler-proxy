package proxy

import (
	"context"
	"io"
	"net"
	"sync"
	"time"

	as "github.com/JonasBak/docker-api-autoscaler-proxy/autoscaler"
	"github.com/JonasBak/docker-api-autoscaler-proxy/utils"
	"github.com/sirupsen/logrus"
)

var log = utils.Logger().WithField("pkg", "proxy")

type Proxy struct {
	as         as.Autoscaler
	listenAddr string
	wg         *sync.WaitGroup
}

func New(addr string) Proxy {
	return Proxy{
		as:         as.New(),
		listenAddr: "127.0.0.1:8081",
		wg:         &sync.WaitGroup{},
	}
}

func (p Proxy) acceptIncoming(l net.Listener, log *logrus.Entry) chan net.Conn {
	newConns := make(chan net.Conn)

	go func(l net.Listener) {
		for {
			c, err := l.Accept()
			if err != nil {
				log.WithError(err).Error("Error accepting incoming request")
				break
			}
			log.WithField("remote_addr", c.RemoteAddr().String()).Debug("Accepted request")
			newConns <- c
		}
	}(l)

	return newConns
}

func (p Proxy) handleRequest(ctx context.Context, c net.Conn, log *logrus.Entry) {
	log.Debug("Handling request")

	ctx2, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	defer c.Close()

	upstream, err := p.as.GetConnection(ctx2)
	if err != nil {
		log.WithError(err).Error("Failed to connect to autoscaler upstream")
		return
	}
	defer upstream.Close()

	stop := make(chan struct{})

	go func() {
		io.Copy(upstream, c)
		stop <- struct{}{}
	}()
	go func() {
		io.Copy(c, upstream)
		stop <- struct{}{}
	}()

	<-stop

	log.Debug("Request handled")
}

func (p Proxy) Start(ctx context.Context) error {
	log := log.WithField("addr", p.listenAddr)
	log.Info("Listen to incoming requests")

	p.wg.Add(1)
	defer p.wg.Done()

	l, err := (&net.ListenConfig{}).Listen(ctx, "tcp", p.listenAddr)
	defer l.Close()

	if err != nil {
		log.WithError(err).Fatal("Failed to listen")
	}

	newConns := p.acceptIncoming(l, log)

LOOP:
	for {
		select {
		case c := <-newConns:
			log := log.WithField("remote_addr", c.RemoteAddr().String())

			if err := p.as.EnsureOnline(ctx); err != nil {
				c.Close()
				log.WithError(err).Error("Autoscaler ensure online failed")
				continue LOOP
			}

			p.wg.Add(1)
			go func() {
				defer p.wg.Done()
				p.handleRequest(ctx, c, log)
			}()
			break
		case <-ctx.Done():
			break LOOP
		}
	}

	return nil
}

func (p Proxy) Stop() {
	log.Debug("Stopping proxy...")

	log.Debug("Waiting for all requests to finish")
	p.wg.Wait()

	log.Debug("Shutting down autoscaler")
	p.as.Shutdown()

	log.Debug("Done")
}
