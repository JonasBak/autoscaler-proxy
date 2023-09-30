package utils

import (
	"os"
	"reflect"
	"regexp"
	"strings"
)

var re = regexp.MustCompile(`\${.+?}`)

type TemplateFunc = func(string) *string

func buildTemplateString(templateFunc TemplateFunc, v string) string {
	v = string(re.ReplaceAllFunc([]byte(v), func(match []byte) []byte {
		key := string(match[2 : len(match)-1])
		r := templateFunc(key)
		if r == nil {
			return match
		}
		return []byte(*r)
	}))
	return v
}

func buildTemplateValue(templateFunc TemplateFunc, v interface{}) interface{} {
	ty := reflect.TypeOf(v).Kind()
	if ty == reflect.String {
		return buildTemplateString(templateFunc, v.(string))
	} else if ty == reflect.Slice {
		s := reflect.ValueOf(v)
		for i := 0; i < s.Len(); i++ {
			templated := buildTemplateValue(templateFunc, s.Index(i).Interface())
			s.Index(i).Set(reflect.ValueOf(templated))
		}
		return v
	} else if ty == reflect.Map {
		v2 := v
		iter := reflect.ValueOf(v).MapRange()
		for iter.Next() {
			templated := buildTemplateValue(templateFunc, iter.Value().Interface())
			reflect.ValueOf(v2).SetMapIndex(iter.Key(), reflect.ValueOf(templated))
		}
		return v2
	}
	return v
}

func BuildTemplate(templateFunc TemplateFunc, template interface{}) interface{} {
	config := buildTemplateValue(templateFunc, template)

	return config
}

func TemplateMap(m map[string]string) TemplateFunc {
	return func(key string) *string {
		v, ok := m[key]
		if !ok {
			return nil
		}
		return &v
	}
}

func WithEnvMap(f TemplateFunc) TemplateFunc {
	return func(key string) *string {
		if strings.HasPrefix(key, "env.") {
			v := os.Getenv(key[4:])
			return &v
		}
		return f(key)
	}
}
