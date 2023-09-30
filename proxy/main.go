package proxy

import (
	"context"
	"io"
	"net"
	"sync"
	"time"

	as "github.com/JonasBak/autoscaler-proxy/autoscaler"
	"github.com/JonasBak/autoscaler-proxy/procs"
	"github.com/JonasBak/autoscaler-proxy/utils"
)

var log = utils.Logger().WithField("pkg", "proxy")

type newConnectionCallback struct {
	addr string
	conn net.Conn
}

type ProxyOpts struct {
	Autoscaler as.AutoscalerOpts          `yaml:"autoscaler"`
	ListenAddr map[string]as.UpstreamOpts `yaml:"listen_addr"`
	Procs      procs.ProcsOpts            `yaml:"procs"`
}

type Proxy struct {
	as         as.Autoscaler
	listenAddr map[string]as.UpstreamOpts
	procs      procs.Procs
	// Used to keep track of ongoing connections, and wait for them to close when
	// stopping the proxy.
	wg *sync.WaitGroup
}

func New(opts ProxyOpts) Proxy {
	return Proxy{
		as:         as.New(opts.Autoscaler),
		listenAddr: opts.ListenAddr,
		procs:      procs.New(opts.Procs),
		wg:         &sync.WaitGroup{},
	}
}

// Spawns a goroutine that accepts incoming connections, and sends them, along with
// addr to c, to keep track of which addr the connection came from.
func (p Proxy) acceptIncoming(ctx context.Context, addr string, c chan newConnectionCallback) {
	log := log.WithField("addr", addr)
	log.Debug("Setting up listener at addr")

	l, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		log.WithError(err).Error("Error listening to addr")
		return
	}
	defer l.Close()
	go func() {
		<-ctx.Done()
		l.Close()
	}()

	log.Info("Listening at addr")

	for {
		conn, err := l.Accept()
		if err != nil {
			log.WithError(err).Error("Error accepting incoming request")
			return
		}
		log.WithField("remote_addr", conn.RemoteAddr().String()).Debug("Accepted request")
		c <- newConnectionCallback{
			addr: addr,
			conn: conn,
		}
	}
}

// Takes an incoming connection and the addr it came at, gets the appropriate connection from
// the autoscaler based on addr, and "connects" the two.
func (p Proxy) handleRequest(ctx context.Context, c net.Conn, addr string) {
	log := log.WithField("remote_addr", c.RemoteAddr().String())
	log.Debug("Handling request")

	// Route the connection based on which addr it came from
	upstreamOpts := p.listenAddr[addr]

	ctx2, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	defer c.Close()

	upstream, err := p.as.GetConnection(ctx2, upstreamOpts)
	if err != nil {
		log.WithError(err).Error("Failed to connect to autoscaler upstream")
		return
	}
	defer upstream.Close()

	stop := make(chan struct{}, 2)

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

// Blocking function that starts the autoscaler and listens and handles incoming requests.
func (p Proxy) Start(ctx context.Context) error {
	log.Info("Starting proxy")

	// Keep track of this "main" goroutine
	p.wg.Add(1)
	defer p.wg.Done()

	// This is not keeped track of as it is "manually" stopped by calling Shutdown()
	// in Stop
	go func() {
		p.as.Start(ctx)
	}()

	newConns := make(chan newConnectionCallback)

	for addr := range p.listenAddr {
		addr := addr
		// Keep track of each listener goroutine
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.acceptIncoming(ctx, addr, newConns)
		}()
	}

	p.procs.Run()

LOOP:
	for {
		select {
		case c := <-newConns:
			if err := p.as.EnsureOnline(ctx); err != nil {
				c.conn.Close()
				log.WithField("remote_addr", c.conn.RemoteAddr().String()).WithError(err).Error("Autoscaler ensure online failed")
				continue LOOP
			}

			// Keep track of the goroutine that handles the connection
			p.wg.Add(1)
			go func() {
				defer p.wg.Done()
				p.handleRequest(ctx, c.conn, c.addr)
			}()
			break
		case <-ctx.Done():
			break LOOP
		}
	}

	return nil
}

// Try to gracefully stop the proxy and autoscaler.
func (p Proxy) Stop() {
	log.Debug("Stopping proxy...")

	log.Debug("Stopping procs")
	p.procs.Shutdown()

	log.Debug("Waiting for all requests to finish")
	p.wg.Wait()

	log.Debug("Shutting down autoscaler")
	p.as.Shutdown()

	log.Debug("Done")
}

// Forcefully kill the proxy and autoscaler
func (p Proxy) Kill() {
	log.Debug("Killing proxy...")

	log.Debug("Killing procs")
	p.procs.Kill()

	log.Debug("Killing autoscaler")
	p.as.Kill()

	log.Debug("Done")
}
