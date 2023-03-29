package autoscaler

import (
	"fmt"

	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

func CreateCloudInitFile(serverKeyBytes []byte, authorizedKey ssh.PublicKey) (string, error) {
	serverKey, err := ssh.ParsePrivateKey(serverKeyBytes)
	if err != nil {
		return "", err
	}
	pubkeyBytes := ssh.MarshalAuthorizedKey(serverKey.PublicKey())
	authorizedKeyBytes := ssh.MarshalAuthorizedKey(authorizedKey)

	config := map[interface{}]interface{}{
		"groups":     []string{"docker"},
		"ssh_pwauth": false,
		"ssh_keys": map[interface{}]interface{}{
			"rsa_private": string(serverKeyBytes),
			"rsa_public":  string(pubkeyBytes),
		},
		"users": []interface{}{
			"default",
			map[interface{}]interface{}{
				"name":                "autoscaler",
				"groups":              "users,docker",
				"lock_passwd":         true,
				"ssh_authorized_keys": []string{string(authorizedKeyBytes)},
			},
		},
	}

	d, err := yaml.Marshal(&config)

	return fmt.Sprintf("#cloud-config\n%s", d), err
}
