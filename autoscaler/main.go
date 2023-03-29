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
	_ "golang.org/x/crypto/ssh"
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
	client     *hcloud.Client
	mx         *sync.Mutex
	server     *hcloud.Server
	serverOpts hcloud.ServerCreateOpts

	// Used to connect to the server after it has been created. Generates a private key for
	// itself and creates a private key for the server. Both of these are created on Start().
	// Note that this means that one instance of the autoscaler can't talk to servers created
	// by other instances.
	sshClient SSHClient
	// Used to "queue" a scaledown evaluation with EvaluateScaledown
	scaledownDebounce *utils.Debouncer
	// Used to decide when it is time to scale down
	lastInteraction time.Time
	// The max length of time before a connection is forcefully closed. Used to avoid lingering
	// connections keeping the server running.
	connectionTimeout time.Duration
	// How long to wait after lastInteraction before scaling down. Should be greater than
	// connectionTimeout.
	scaledownAfter time.Duration
	// Channel used to communicate with the Start thread that it should be scaled up.
	cUp chan chan error
	// Channel used to communicate with the Start thread that it should be scaled down.
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

	sshClient := newSSHClient()

	// serverOpts := serverOptions(client, opts)

	as := Autoscaler{
		mx:     new(sync.Mutex),
		client: client,
		// serverOpts: serverOpts,

		sshClient: sshClient,

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
// and check in later. This version of the function should only be called from
// the goroutine running Start(), others should use EvaluateScaledown.
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

// Threadsafe version of evaluateScaledown, idempotent.
func (as *Autoscaler) EvaluateScaledown() {
	as.scaledownDebounce.F()
}

// This function should be called before GetConnection to ensure that the
// server is online before trying to connect to it. This version
// of the function should only be called from the goroutine running Start(),
// other should use EnsureOnline.
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

// Threadsafe version of ensureOnline, idempotent.
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

// Starts the autoscaler. This is blocking and should be started in its own goroutine.
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
