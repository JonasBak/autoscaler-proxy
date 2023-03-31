package main

import (
	"os"
	"time"

	as "github.com/JonasBak/docker-api-autoscaler-proxy/autoscaler"
	"github.com/JonasBak/docker-api-autoscaler-proxy/proxy"
	"go.mozilla.org/sops/v3/decrypt"
	"gopkg.in/yaml.v3"
)

func DefaultConfig() proxy.ProxyOpts {
	return proxy.ProxyOpts{
		Autoscaler: as.AutoscalerOpts{
			ConnectionTimeout: 10 * time.Minute,
			ScaledownAfter:    15 * time.Minute,

			ServerNamePrefix: "autoscaler",
			ServerType:       "cpx31",
			ServerImage:      "docker-ce",

			CloudInitTemplate: map[string]interface{}{
				"groups":     []string{"docker"},
				"ssh_pwauth": false,
				"ssh_keys": map[string]string{
					"rsa_private": "${SERVER_RSA_PRIVATE}",
					"rsa_public":  "${SERVER_RSA_PUBLIC}",
				},
				"users": []interface{}{
					"default",
					map[string]interface{}{
						"name":                "autoscaler",
						"groups":              "users,docker",
						"lock_passwd":         true,
						"ssh_authorized_keys": []string{"${AUTOSCALER_AUTHORIZED_KEY}"},
					},
				},
			},
			CloudInitVariables: map[string]string{},
		},
		ListenAddr: map[string]as.UpstreamOpts{
			"127.0.0.1:8081": {
				Net:  "unix",
				Addr: "/var/run/docker.sock",
			},
		},
	}
}

func ParseConfigFile(path string) (proxy.ProxyOpts, error) {
	opts := DefaultConfig()

	file, err := os.ReadFile(path)
	if err != nil {
		return opts, err
	}

	if err := yaml.Unmarshal(file, &opts); err != nil {
		return opts, err
	}

	if opts.Autoscaler.CloudInitVariablesFrom != "" {
		variables, err := decrypt.File(opts.Autoscaler.CloudInitVariablesFrom, "yaml")
		if err != nil {
			return opts, err
		}
		if err := yaml.Unmarshal(variables, &opts.Autoscaler.CloudInitVariables); err != nil {
			return opts, err
		}
	}

	return opts, nil
}
