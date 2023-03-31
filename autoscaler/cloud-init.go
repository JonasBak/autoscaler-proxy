package autoscaler

import (
	"fmt"
	"reflect"
	"strings"

	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

func buildCloudInitTemplateString(variables map[string]string, v string) string {
	for key, replaceWith := range variables {
		v = strings.ReplaceAll(v, fmt.Sprintf("${%s}", key), replaceWith)
	}
	return v
}

func buildCloudInitTemplateSlice(variables map[string]string, v []interface{}) []interface{} {
	for i, elem := range v {
		v[i] = buildCloudInitTemplateValue(variables, elem)
	}
	return v
}

func buildCloudInitTemplateValue(variables map[string]string, v interface{}) interface{} {
	ty := reflect.TypeOf(v).Kind()
	if ty == reflect.String {
		return buildCloudInitTemplateString(variables, v.(string))
	} else if ty == reflect.Slice {
		s := reflect.ValueOf(v)
		v2 := []interface{}{}
		for i := 0; i < s.Len(); i++ {
			v2 = append(v2, s.Index(i).Interface())
		}
		return buildCloudInitTemplateSlice(variables, v2)
	} else if ty == reflect.Map {
		v2 := map[string]interface{}{}
		iter := reflect.ValueOf(v).MapRange()
		for iter.Next() {
			v2[iter.Key().Interface().(string)] = iter.Value().Interface()
		}
		return buildCloudInitTemplate(variables, v2)
	}
	return v
}

func buildCloudInitTemplate(variables map[string]string, template map[string]interface{}) map[string]interface{} {
	for k, v := range template {
		template[k] = buildCloudInitTemplateValue(variables, v)
	}
	return template
}

func CreateCloudInitFile(template map[string]interface{}, variables map[string]string, serverKeyBytes []byte, authorizedKey ssh.PublicKey) (string, error) {
	serverKey, err := ssh.ParsePrivateKey(serverKeyBytes)
	if err != nil {
		return "", err
	}
	pubkeyBytes := ssh.MarshalAuthorizedKey(serverKey.PublicKey())
	authorizedKeyBytes := ssh.MarshalAuthorizedKey(authorizedKey)

	if variables == nil {
		variables = make(map[string]string)
	}

	variables["SERVER_RSA_PRIVATE"] = string(serverKeyBytes)
	variables["SERVER_RSA_PUBLIC"] = string(pubkeyBytes)
	variables["AUTOSCALER_AUTHORIZED_KEY"] = string(authorizedKeyBytes)

	config := buildCloudInitTemplate(variables, template)

	d, err := yaml.Marshal(&config)

	return fmt.Sprintf("#cloud-config\n%s", d), err
}
