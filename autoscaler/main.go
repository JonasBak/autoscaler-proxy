package autoscaler

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/JonasBak/docker-api-autoscaler-proxy/utils"
	"github.com/hetznercloud/hcloud-go/hcloud"
)

var log = utils.Logger().WithField("pkg", "autoscaler")

type AutoscalerOpts struct {
	ConnectionTimeout time.Duration
	ScaledownAfter    time.Duration

	ServerNamePrefix string
	ServerType       string
	ServerImage      string
}

func serverOptions(client *hcloud.Client, opts AutoscalerOpts) hcloud.ServerCreateOpts {
	serverType, _, err := client.ServerType.GetByName(context.Background(), opts.ServerType)
	if err != nil {
		log.WithError(err).WithField("server_type", opts.ServerType).Fatal("Failed to fetch hetzner server type")
	}

	image, _, err := client.Image.GetByName(context.Background(), opts.ServerImage)
	if err != nil {
		log.WithError(err).WithField("server_image", opts.ServerImage).Fatal("Failed to fetch hetzner image")
	}

	return hcloud.ServerCreateOpts{
		Name:       fmt.Sprintf("%s_todo", opts.ServerNamePrefix),
		ServerType: serverType,
		Image:      image,

		UserData: "TODO cloud-init",

		// TODO
	}
}

type Autoscaler struct {
	client *hcloud.Client

	mx     *sync.Mutex
	server *hcloud.Server

	serverOpts hcloud.ServerCreateOpts

	// Used to "queue" a scaledown evaluation with EvaluateScaledown
	scaledownDebounce *utils.Debouncer
	// Used to decide when it is time to scale down
	lastInteraction time.Time

	connectionTimeout time.Duration
	scaledownAfter    time.Duration

	cUp   chan chan error
	cDown chan struct{}
}

func New() Autoscaler {
	opts := AutoscalerOpts{
		ConnectionTimeout: 20 * time.Second,
		ScaledownAfter:    25 * time.Second,

		ServerNamePrefix: "autoscaler",
		ServerType:       "TODO",
		ServerImage:      "TODO",
	}

	client := hcloud.NewClient(hcloud.WithToken("token"))

	// serverOpts := serverOptions(client, opts)

	as := Autoscaler{
		mx:     new(sync.Mutex),
		client: client,
		// serverOpts: serverOpts,

		lastInteraction: time.Now(),

		connectionTimeout: opts.ConnectionTimeout,
		scaledownAfter:    opts.ScaledownAfter,

		cUp:   make(chan chan error),
		cDown: make(chan struct{}),
	}

	as.scaledownDebounce = utils.NewDebouncer(5*time.Second, func() {
		as.cDown <- struct{}{}
	})

	return as
}

func (as *Autoscaler) createServer() error {
	if as.server != nil {
		return fmt.Errorf("Server already exists")
	}

	log.Info("Creating server")

	return nil
}

func (as *Autoscaler) deleteServer() error {
	// if as.server == nil {
	// 	return nil
	// }

	log.Info("Deleting server")

	return nil
}

// This function will check if it is time to scale down. This decision is based
// on the time since last (EnsureOnline) interaction with the autoscaler.
// If it isn't time to scale down yet, the function will "re-queue" itself,
// and check in later.
func (as *Autoscaler) evaluateScaledown(ctx context.Context) {
	log.Debug("Evaluating scaledown")
	// if as.server == nil {
	// 	return nil
	// }

	as.mx.Lock()
	defer as.mx.Unlock()

	timeSinceLastInteraction := time.Now().Sub(as.lastInteraction)

	if timeSinceLastInteraction <= as.scaledownAfter {
		as.scaledownDebounce.F()
		return
	}

	as.deleteServer()
}

func (as *Autoscaler) EvaluateScaledown() {
	as.scaledownDebounce.F()
}

// This function should be called before GetConnection to ensure that the
// server is online before trying to connect to it, can be called many times.
func (as *Autoscaler) ensureOnline(ctx context.Context) error {
	log.Debug("Making sure server is online")
	as.mx.Lock()
	defer as.mx.Unlock()

	as.lastInteraction = time.Now()

	if as.server == nil {
		log.Info("No server online, will be created")
		err := as.createServer()
		if err != nil {
			return err
		}
	}

	as.scaledownDebounce.F()

	return nil
}

func (as *Autoscaler) EnsureOnline(ctx context.Context) error {
	c := make(chan error)
	as.cUp <- c
	return <-c
}

func (as *Autoscaler) GetConnection(ctx context.Context) (io.ReadWriteCloser, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", "127.0.0.1:8082")
	rwc, c := utils.NewReadWriteCloseNotifier(conn)

	if err == nil {
		go func() {
			select {
			case <-time.NewTimer(as.connectionTimeout).C:
				log.Warn("Connection has been open for too long, closing")
				rwc.Close()
			case <-c:
				as.EvaluateScaledown()
				break
			}
		}()
	}

	return rwc, err
}

func (as *Autoscaler) Start(ctx context.Context) {
	log.Info("Starting autoscaler")
LOOP:
	for {
		select {
		case c := <-as.cUp:
			c <- as.ensureOnline(ctx)
			break
		case <-as.cDown:
			as.evaluateScaledown(ctx)
			break
		case <-ctx.Done():
			break LOOP
		}
	}
}

func (as *Autoscaler) Shutdown() {
}
