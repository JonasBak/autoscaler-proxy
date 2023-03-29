package autoscaler

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"golang.org/x/crypto/ssh"
)

var RSA_KEY_BITS = 4096

type SSHClient struct {
	// Config set up to connect using publicKey to a server holding remoteKey
	config ssh.ClientConfig
	// Public key for the key used to authenticate to the server
	publicKey ssh.PublicKey
	// Private key (PEM) to be put on the server, only key that this client will
	// accept when connecting.
	remoteKey []byte
}

// Creates an SSHClient and generates a pair of rsa keys, one for the client
// and one for the server that can be distributed using cloud-init.
func newSSHClient() SSHClient {
	log.Debug("Generating local ssh key")
	key, err := generatePrivateKey()
	if err != nil {
		log.WithError(err).Fatal("Faled to generate local ssh key")
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		log.WithError(err).Fatal("Faled to parse local ssh key")
	}

	log.Debug("Generating remote ssh key")
	remoteKey, err := generatePrivateKey()
	if err != nil {
		log.WithError(err).Fatal("Faled to generate remote ssh key")
	}
	remoteSigner, err := ssh.ParsePrivateKey(remoteKey)
	if err != nil {
		log.WithError(err).Fatal("Faled to parse remote ssh key")
	}

	return SSHClient{
		config: ssh.ClientConfig{
			User: "autoscaler",
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			},
			HostKeyCallback:   ssh.FixedHostKey(remoteSigner.PublicKey()),
			HostKeyAlgorithms: []string{ssh.KeyAlgoRSASHA512},
		},
		publicKey: signer.PublicKey(),
		remoteKey: remoteKey,
	}
}

// Connect to sshAddr using credentials and configuration from the SSHClient
func (c SSHClient) Connect(sshAddr string) (*ssh.Client, error) {
	conn, err := ssh.Dial("tcp", sshAddr, &c.config)
	if err != nil {
		return nil, err
	}

	return conn, err
}

func generatePrivateKey() ([]byte, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, RSA_KEY_BITS)
	if err != nil {
		return nil, err
	}

	err = privateKey.Validate()
	if err != nil {
		return nil, err
	}

	privDER := x509.MarshalPKCS1PrivateKey(privateKey)

	privBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDER,
	}

	privatePEM := pem.EncodeToMemory(&privBlock)

	return privatePEM, nil
}
