package autoscaler

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/JonasBak/autoscaler-proxy/utils"
	"github.com/hetznercloud/hcloud-go/hcloud"
)

var log = utils.Logger().WithField("pkg", "autoscaler")

type UpstreamOpts struct {
	Net  string `yaml:"net"`
	Addr string `yaml:"addr"`
}

type AutoscalerOpts struct {
	HCloudToken string `yaml:"hcloud_token"`

	ConnectionTimeout time.Duration `yaml:"connection_timeout"`
	ScaledownAfter    time.Duration `yaml:"scaledown_after"`

	ServerNamePrefix string `yaml:"server_name_prefix"`
	ServerType       string `yaml:"server_type"`
	ServerImage      string `yaml:"server_image"`
	ServerLocation   string `yaml:"server_location"`

	WaitFor *UpstreamOpts `yaml:"wait_for"`

	CloudInitTemplate      map[string]interface{} `yaml:"cloud_init_template"`
	CloudInitVariables     map[string]string      `yaml:"cloud_init_variables"`
	CloudInitVariablesFrom string                 `yaml:"cloud_init_variables_from"`
}

func serverOptions(client *hcloud.Client, opts AutoscalerOpts, cloudInit string) hcloud.ServerCreateOpts {
	serverType, _, err := client.ServerType.GetByName(context.Background(), opts.ServerType)
	if err != nil {
		log.WithError(err).WithField("server_type", opts.ServerType).Fatal("Failed to fetch hetzner server type")
	}

	image, _, err := client.Image.GetByName(context.Background(), opts.ServerImage)
	if err != nil {
		log.WithError(err).WithField("server_image", opts.ServerImage).Fatal("Failed to fetch hetzner server image")
	}

	var location *hcloud.Location = nil
	if opts.ServerLocation != "" {
		l, _, err := client.Location.GetByName(context.Background(), opts.ServerLocation)
		if err != nil {
			log.WithError(err).WithField("server_location", opts.ServerLocation).Fatal("Failed to fetch hetzner server location")
		}
		location = l
	}

	name := fmt.Sprintf("%s-%s", opts.ServerNamePrefix, utils.RandomString(6))

	log = log.WithField("server", name)

	return hcloud.ServerCreateOpts{
		Name:       name,
		ServerType: serverType,
		Image:      image,
		Location:   location,

		UserData: cloudInit,
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
	// Channel used to communicate with the Start thread that it should be shut down
	cShutdown chan chan error

	waitFor *UpstreamOpts
}

func New(opts AutoscalerOpts) Autoscaler {
	client := hcloud.NewClient(hcloud.WithToken(opts.HCloudToken))

	sshClient := newSSHClient()

	cloudInit, err := CreateCloudInitFile(opts.CloudInitTemplate, opts, sshClient.remoteKey, sshClient.publicKey)
	if err != nil {
		log.WithError(err).Fatal("Failed to generate cloud-init.yml")
	}

	serverOpts := serverOptions(client, opts, cloudInit)

	as := Autoscaler{
		mx:                new(sync.Mutex),
		client:            client,
		serverOpts:        serverOpts,
		sshClient:         sshClient,
		lastInteraction:   time.Now(),
		connectionTimeout: opts.ConnectionTimeout,
		scaledownAfter:    opts.ScaledownAfter,
		cUp:               make(chan chan error),
		cShutdown:         make(chan chan error),
		waitFor:           opts.WaitFor,
	}

	return as
}

func (as *Autoscaler) createServer() error {
	if as.server != nil {
		return fmt.Errorf("Server already exists")
	}

	log.Info("Creating server")

	result, _, err := as.client.Server.Create(context.Background(), as.serverOpts)
	if err != nil {
		log.WithError(err).Error("Failed to create server")
		return err
	}

	log.Info("Waiting for server to start")
	_, c := as.client.Action.WatchProgress(context.Background(), result.Action)

	err = <-c
	if err != nil {
		log.WithError(err).Error("Failed to start server")
		return err
	}

	log.Info("Server created")

	as.server = result.Server

	return nil
}

func (as *Autoscaler) deleteServer() error {
	if as.server == nil {
		return nil
	}

	log.Info("Deleting server")

	_, err := as.client.Server.Delete(context.Background(), as.server)
	if err != nil {
		log.WithError(err).Error("Failed to delete server")
		return err
	}

	log.Info("Server deleted")

	as.server = nil

	return nil
}

// This function will check if it is time to scale down. This decision is based
// on the time since last (EnsureOnline) interaction with the autoscaler.
// This version of the function should only be called from the goroutine running Start().
func (as *Autoscaler) evaluateScaledown(ctx context.Context) error {
	if as.server == nil {
		return nil
	}

	as.mx.Lock()
	defer as.mx.Unlock()

	timeSinceLastInteraction := time.Now().Sub(as.lastInteraction)

	log.WithField("time_since_last_interaction", timeSinceLastInteraction.String()).Debug("Evaluating scaledown")

	if timeSinceLastInteraction <= as.scaledownAfter {
		return nil
	}

	return as.deleteServer()
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

		log.Info("Waiting for ping")
		err = ping(6, 4, 5, as.server.PublicNet.IPv4.IP.String()+":22")
		if err != nil {
			return err
		}
		if waitFor := as.waitFor; waitFor != nil {
			log.Info("Pinging wait_for")
			sshConn, err := as.sshClient.Connect(as.server.PublicNet.IPv4.IP.String() + ":22")
			if err != nil {
				return err
			}
			defer sshConn.Close()
			pingConn(6, 5, func() (net.Conn, error) {
				return sshConn.Dial(waitFor.Net, waitFor.Addr)
			})
		}
	} else {
		err := ping(2, 2, 1, as.server.PublicNet.IPv4.IP.String()+":22")
		if err != nil {
			return err
		}

	}

	return nil
}

// Threadsafe version of ensureOnline, idempotent.
func (as *Autoscaler) EnsureOnline(ctx context.Context) error {
	c := make(chan error)
	as.cUp <- c
	return <-c
}

func (as *Autoscaler) GetConnection(ctx context.Context, opts UpstreamOpts) (io.ReadWriteCloser, error) {
	// TODO Could share one ssh connection?
	sshConn, err := as.sshClient.Connect(as.server.PublicNet.IPv4.IP.String() + ":22")
	if err != nil {
		return nil, err
	}
	conn, err := sshConn.Dial(opts.Net, opts.Addr)
	rwc, c := utils.NewReadWriteCloseNotifier(conn)

	if err == nil {
		go func() {
			defer sshConn.Close()
			select {
			case <-time.NewTimer(as.connectionTimeout).C:
				log.Warn("Connection has been open for too long, closing")
				rwc.Close()
			case <-c:
				break
			}
		}()
	}

	return rwc, err
}

// Starts the autoscaler. This is blocking and should be started in its own goroutine.
func (as *Autoscaler) Start(ctx context.Context) {
	log.Info("Starting autoscaler")

	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
LOOP:
	for {
		select {
		case c := <-as.cUp:
			err := as.ensureOnline(ctx)
			if err != nil {
				log.WithError(err).Error("Failed ensure online")
			}
			c <- err
			break
		case <-ticker.C:
			err := as.evaluateScaledown(ctx)
			if err != nil {
				log.WithError(err).Error("Failed evaluate scaledown")
			}
			break
		case c := <-as.cShutdown:
			as.mx.Lock()
			defer as.mx.Unlock()
			c <- as.deleteServer()
			break LOOP
		}
	}
}

func (as *Autoscaler) Shutdown() {
	c := make(chan error)
	as.cShutdown <- c

	err := <-c
	if err != nil {
		log.WithError(err).Error("Failed do shut down autoscaler")
	}
}
