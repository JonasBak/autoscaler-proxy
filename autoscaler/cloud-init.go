package autoscaler

import (
	"fmt"
	"reflect"
	"strings"

	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

func buildCloudInitTemplateString(replace map[string]string, v string) string {
	for key, replaceWith := range replace {
		v = strings.ReplaceAll(v, fmt.Sprintf("${%s}", key), replaceWith)
	}
	return v
}

func buildCloudInitTemplateSlice(replace map[string]string, v []interface{}) []interface{} {
	for i, elem := range v {
		v[i] = buildCloudInitTemplateValue(replace, elem)
	}
	return v
}

func buildCloudInitTemplateValue(replace map[string]string, v interface{}) interface{} {
	ty := reflect.TypeOf(v).Kind()
	if ty == reflect.String {
		return buildCloudInitTemplateString(replace, v.(string))
	} else if ty == reflect.Slice {
		s := reflect.ValueOf(v)
		v2 := []interface{}{}
		for i := 0; i < s.Len(); i++ {
			v2 = append(v2, s.Index(i).Interface())
		}
		return buildCloudInitTemplateSlice(replace, v2)
	} else if ty == reflect.Map {
		v2 := map[string]interface{}{}
		iter := reflect.ValueOf(v).MapRange()
		for iter.Next() {
			v2[iter.Key().Interface().(string)] = iter.Value().Interface()
		}
		return buildCloudInitTemplate(replace, v2)
	}
	return v
}

func buildCloudInitTemplate(replace map[string]string, template map[string]interface{}) map[string]interface{} {
	for k, v := range template {
		template[k] = buildCloudInitTemplateValue(replace, v)
	}
	return template
}

func CreateCloudInitFile(template map[string]interface{}, serverKeyBytes []byte, authorizedKey ssh.PublicKey) (string, error) {
	serverKey, err := ssh.ParsePrivateKey(serverKeyBytes)
	if err != nil {
		return "", err
	}
	pubkeyBytes := ssh.MarshalAuthorizedKey(serverKey.PublicKey())
	authorizedKeyBytes := ssh.MarshalAuthorizedKey(authorizedKey)

	replace := map[string]string{
		"SERVER_RSA_PRIVATE":        string(serverKeyBytes),
		"SERVER_RSA_PUBLIC":         string(pubkeyBytes),
		"AUTOSCALER_AUTHORIZED_KEY": string(authorizedKeyBytes),
	}
	config := buildCloudInitTemplate(replace, template)

	d, err := yaml.Marshal(&config)

	return fmt.Sprintf("#cloud-config\n%s", d), err
}
