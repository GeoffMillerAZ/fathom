// Bench-only native Kong typed-population probe; no product dependency.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/blang/semver/v4"
	"github.com/kong/go-database-reconciler/pkg/dump"
	"github.com/kong/go-database-reconciler/pkg/file"
	"github.com/kong/go-database-reconciler/pkg/state"
)

const maxSource = 1 << 20
const maxEntities = 10000
const maxOutput = 8 << 20

type denyTransport struct{ calls *int }

func (d denyTransport) RoundTrip(*http.Request) (*http.Response, error) {
	*d.calls++
	return nil, fmt.Errorf("native probe refuses every HTTP request")
}

type rawService struct{ name, id, host, url *string }
type node struct {
	Kind        string  `json:"kind"`
	Name        string  `json:"name,omitempty"`
	ID          string  `json:"written_id,omitempty"`
	Scope       string  `json:"scope,omitempty"`
	WrittenHost *string `json:"written_host,omitempty"`
	WrittenURL  *string `json:"written_url,omitempty"`
}
type reference struct {
	FromKind    string `json:"from_kind"`
	From        string `json:"from"`
	Scope       string `json:"scope,omitempty"`
	Field       string `json:"field"`
	ToKind      string `json:"to_kind"`
	WrittenName string `json:"written_name,omitempty"`
	WrittenID   string `json:"written_id,omitempty"`
	Outcome     string `json:"outcome"`
	To          string `json:"to,omitempty"`
}
type result struct {
	Tool             string      `json:"tool"`
	Version          string      `json:"version"`
	Status           string      `json:"status"`
	Nodes            []node      `json:"native_entities"`
	References       []reference `json:"native_written_fields"`
	ServiceCallbacks int         `json:"native_service_callbacks"`
	HTTPRequests     int         `json:"http_requests"`
	Error            string      `json:"error,omitempty"`
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
func label(kind, name, id string) string {
	if name != "" {
		return kind + ":" + name
	}
	return kind + ":id:" + id
}

func observe(source []byte) (r result) {
	r = result{Tool: "Kong native typed loader/resolver probe", Version: "go-database-reconciler v1.42.4", Status: "refused"}
	defer func() {
		if value := recover(); value != nil {
			r.Status = "refused"
			r.Error = fmt.Sprint("native panic: ", value)
		}
	}()
	transport := http.DefaultTransport
	http.DefaultTransport = denyTransport{&r.HTTPRequests}
	defer func() { http.DefaultTransport = transport }()
	if len(source) > maxSource {
		r.Error = "whole-input source cap exceeded"
		return
	}
	var raw []rawService
	file.OracleServiceObserver = func(name, id, host, url *string) { raw = append(raw, rawService{name, id, host, url}) }
	defer func() { file.OracleServiceObserver = nil }()
	content, err := file.GetContentFromReader(bytes.NewReader(source), file.EnvVarsMock)
	r.ServiceCallbacks = len(raw)
	if err != nil {
		r.Error = err.Error()
		return
	}
	// Match the native decoded pointer fields that copyFromService assigns
	// unchanged. Names may repeat; pointer identity prevents a name-based join.
	matched := make([]bool, len(raw))
	count := 0
	add := func(n node) { r.Nodes = append(r.Nodes, n); count++ }
	addRef := func(fromKind, name, scope, field, toKind string, namePtr, idPtr *string) {
		if namePtr == nil && idPtr == nil {
			return
		}
		r.References = append(r.References, reference{FromKind: fromKind, From: name, Scope: scope, Field: field, ToKind: toKind, WrittenName: str(namePtr), WrittenID: str(idPtr), Outcome: "unevaluated"})
		count++
	}
	unsupported := false
	var plugins func([]*file.FPlugin, string)
	plugins = func(items []*file.FPlugin, scope string) {
		for _, p := range items {
			if p == nil {
				unsupported = true
				continue
			}
			add(node{Kind: "plugin", Name: str(p.Name), ID: str(p.ID), Scope: scope})
			if p.Service != nil {
				addRef("plugin", str(p.Name), scope, "service", "service", p.Service.Name, p.Service.ID)
			}
			if p.Route != nil {
				addRef("plugin", str(p.Name), scope, "route", "route", p.Route.Name, p.Route.ID)
			}
		}
	}
	var routes func([]*file.FRoute, string)
	routes = func(items []*file.FRoute, scope string) {
		for _, route := range items {
			if route == nil {
				unsupported = true
				continue
			}
			add(node{Kind: "route", Name: str(route.Name), ID: str(route.ID), Scope: scope})
			if route.Service != nil {
				addRef("route", str(route.Name), scope, "service", "service", route.Service.Name, route.Service.ID)
			}
			plugins(route.Plugins, label("route", str(route.Name), str(route.ID)))
		}
	}
	for i := range content.Services {
		service := &content.Services[i]
		index := -1
		for j, record := range raw {
			if !matched[j] && record.name == service.Name && record.id == service.ID {
				if index != -1 {
					unsupported = true
				}
				index = j
			}
		}
		n := node{Kind: "service", Name: str(service.Name), ID: str(service.ID)}
		if index < 0 {
			unsupported = true
		} else {
			matched[index] = true
			n.WrittenHost = raw[index].host
			n.WrittenURL = raw[index].url
			addRef("service", str(service.Name), "", "host", "upstream", raw[index].host, nil)
		}
		add(n)
		scope := label("service", str(service.Name), str(service.ID))
		routes(service.Routes, scope)
		plugins(service.Plugins, scope)
	}
	for _, done := range matched {
		if !done {
			unsupported = true
		}
	}
	for i := range content.Routes {
		routes([]*file.FRoute{&content.Routes[i]}, "")
	}
	for i := range content.Plugins {
		plugins([]*file.FPlugin{&content.Plugins[i]}, "")
	}
	for _, consumer := range content.Consumers {
		plugins(consumer.Plugins, label("consumer", str(consumer.Username), str(consumer.ID)))
	}
	for _, group := range content.ConsumerGroups {
		for _, plugin := range group.Plugins {
			if plugin == nil {
				unsupported = true
				continue
			}
			add(node{Kind: "plugin", Name: str(plugin.Name), ID: str(plugin.ID), Scope: label("consumer_group", str(group.Name), str(group.ID))})
		}
	}

	for _, upstream := range content.Upstreams {
		add(node{Kind: "upstream", Name: str(upstream.Name), ID: str(upstream.ID)})
		for _, target := range upstream.Targets {
			if target == nil {
				unsupported = true
				continue
			}
			add(node{Kind: "target", Name: str(target.Target.Target), ID: str(target.ID), Scope: label("upstream", str(upstream.Name), str(upstream.ID))})
		}
	}
	if count > maxEntities {
		r.Error = "whole-input native population cap exceeded"
		return
	}
	// Konnect holds deployment-routing metadata only. The pinned offline
	// file.Get path does not consume it; preserve the content unchanged and
	// evaluate its Gateway entity fields normally. Service packages remain
	// refused until their nested implementation population is classified.
	if unsupported || len(content.ServicePackages) > 0 {
		r.Error = "unclassified native typed scope; whole input refused"
		return
	}
	current, err := state.NewKongState()
	if err != nil {
		r.Error = err.Error()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Exact offline decK distributed-to-Gateway3x path: static defaults, nil
	// client, Kong3.0 and an empty current owner state; no server or Docker.
	rawState, err := file.Get(ctx, content, file.RenderConfig{CurrentState: current, KongVersion: semver.Version{Major: 3}}, dump.Config{}, nil)
	if err != nil {
		r.Error = err.Error()
		return
	}
	evaluated, err := state.Get(rawState)
	if err != nil {
		r.Error = err.Error()
		return
	}
	for i := range r.References {
		ref := &r.References[i]
		key := ref.WrittenID
		if key == "" {
			key = ref.WrittenName
		}
		switch ref.ToKind {
		case "service":
			target, e := evaluated.Services.Get(key)
			if e == nil {
				ref.To = str(target.Name)
				ref.Outcome = "owner_registry_resolved"
			} else {
				ref.Outcome = "owner_registry_missing"
			}
		case "route":
			target, e := evaluated.Routes.Get(key)
			if e == nil {
				ref.To = str(target.Name)
				ref.Outcome = "owner_registry_resolved"
			} else {
				ref.Outcome = "owner_registry_missing"
			}
		case "upstream":
			target, e := evaluated.Upstreams.Get(key)
			if e == nil && str(target.Name) == key {
				ref.To = str(target.Name)
				ref.Outcome = "owner_registry_resolved"
			} else {
				ref.Outcome = "literal_host_without_named_upstream"
			}
		}
	}
	if r.HTTPRequests != 0 {
		r.Error = "native code attempted HTTP; whole input refused"
		return
	}
	r.Status = "native_authored_probe_complete_ungraded"
	return
}
func encodeResult(output result) ([]byte, error) {
	data, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}
	if len(data) > maxOutput {
		return nil, fmt.Errorf("whole-input output cap exceeded")
	}
	return data, nil
}

func main() {
	source, err := io.ReadAll(io.LimitReader(os.Stdin, maxSource+1))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	output := observe(source)
	data, err := encodeResult(output)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Stdout.Write(append(data, '\n'))
	if output.Status == "refused" {
		os.Exit(1)
	}
}
