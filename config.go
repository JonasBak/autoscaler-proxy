package main

import (
	"fmt"
	"os"
	"time"

	as "github.com/JonasBak/autoscaler-proxy/autoscaler"
	"github.com/JonasBak/autoscaler-proxy/proxy"
	"github.com/JonasBak/autoscaler-proxy/utils"
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
		ListenAddr: map[string]as.UpstreamOpts{},
	}
}

func patchProcsOpts(opts proxy.ProxyOpts) proxy.ProxyOpts {
	variables := make(map[string]string)
	for addr, upstream := range opts.ListenAddr {
		if upstream.Name != nil {
			variables[fmt.Sprintf("autoscaler.listen.%s", *upstream.Name)] = addr
		}
	}
	env := utils.BuildTemplate(utils.TemplateMap(variables), opts.Procs.Env).(map[string]string)
	opts.Procs.Env = env
	return opts
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

	opts = patchProcsOpts(opts)

	return opts, nil
}
