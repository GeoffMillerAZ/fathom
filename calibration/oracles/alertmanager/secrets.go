package main

import (
	"reflect"
	"sort"
	"strings"

	"github.com/prometheus/alertmanager/config"
)

// secretFields lists, by reflection over the configuration's own types
// (config.Config), every field whose type is a secret: a named type whose
// name begins Secret (prometheus/common's config.Secret, Alertmanager's
// SecretURL and SecretTemplateURL), at any depth (an http_config's
// basic_auth.password, authorization.credentials, oauth2.client_secret,
// tls_config.key, a proxy header's values). Each is a YAML path:
// `.`-joined keys, `[]` a list's item, `{}` a map's value; a route's routes
// are walked once. TestIngest_AlertmanagerSecretsNeverStored plants a
// sentinel in every one.
func secretFields() []string {
	set := map[string]bool{}
	walkSecrets(reflect.TypeOf(config.Config{}), "", map[reflect.Type]int{}, set)
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func isSecretType(t reflect.Type) bool {
	return t.Name() != "" && strings.HasPrefix(t.Name(), "Secret") && strings.Contains(t.PkgPath(), "prometheus/")
}

func walkSecrets(t reflect.Type, p string, seen map[reflect.Type]int, set map[string]bool) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if isSecretType(t) {
		set[p] = true
		return
	}
	switch t.Kind() {
	case reflect.Struct:
		if seen[t] > 1 {
			return
		}
		seen[t]++
		defer func() { seen[t]-- }()
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			tag := f.Tag.Get("yaml")
			name, opts, _ := strings.Cut(tag, ",")
			if name == "-" {
				continue
			}
			if strings.Contains(opts, "inline") || (f.Anonymous && name == "") {
				walkSecrets(f.Type, p, seen, set)
				continue
			}
			if name == "" {
				name = strings.ToLower(f.Name)
			}
			walkSecrets(f.Type, join(p, name), seen, set)
		}
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			return
		}
		walkSecrets(t.Elem(), p+"[]", seen, set)
	case reflect.Map:
		walkSecrets(t.Elem(), p+"{}", seen, set)
	}
}

func join(p, k string) string {
	if p == "" {
		return k
	}
	return p + "." + k
}
