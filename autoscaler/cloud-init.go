package autoscaler

import (
	"fmt"

	"github.com/JonasBak/autoscaler-proxy/utils"
	"go.mozilla.org/sops/v3/decrypt"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

func CreateCloudInitFile(template map[string]interface{}, opts AutoscalerOpts, serverKeyBytes []byte, authorizedKey ssh.PublicKey) (string, error) {
	serverKey, err := ssh.ParsePrivateKey(serverKeyBytes)
	if err != nil {
		return "", err
	}
	pubkeyBytes := ssh.MarshalAuthorizedKey(serverKey.PublicKey())
	authorizedKeyBytes := ssh.MarshalAuthorizedKey(authorizedKey)

	variables := opts.CloudInitVariables
	if variables == nil {
		variables = make(map[string]string)
	}

	if opts.CloudInitVariablesFrom != "" {
		yml, err := decrypt.File(opts.CloudInitVariablesFrom, "yaml")
		if err != nil {
			return "", err
		}
		if err := yaml.Unmarshal(yml, &variables); err != nil {
			return "", err
		}
	}

	variables["SERVER_RSA_PRIVATE"] = string(serverKeyBytes)
	variables["SERVER_RSA_PUBLIC"] = string(pubkeyBytes)
	variables["AUTOSCALER_AUTHORIZED_KEY"] = string(authorizedKeyBytes)

	config := utils.BuildTemplate(utils.TemplateMap(variables), template)

	d, err := yaml.Marshal(&config)

	return fmt.Sprintf("#cloud-config\n%s", d), err
}
