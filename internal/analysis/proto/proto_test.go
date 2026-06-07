package proto_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/analysis/proto"
)

const fixture = `
syntax = "proto3";

package com.example.v1; // trailing comment

// UserService handles user management.
service UserService {
  rpc GetUser (GetUserRequest) returns (GetUserResponse);
  rpc CreateUser (CreateUserRequest) returns (CreateUserResponse);
}

service bad_service {
  rpc get_user (GetUserRequest) returns (GetUserResponse);
}

message GetUserRequest {
  string user_id = 1;
  int32  page_size = 2;
}

message getUserResponse {
  string UserName = 1;
}
`

func TestParse_Package(t *testing.T) {
	c := proto.Parse(fixture)
	if c.Package != "com.example.v1" {
		t.Fatalf("package: got %q, want %q", c.Package, "com.example.v1")
	}
}

func TestParse_Services(t *testing.T) {
	c := proto.Parse(fixture)
	if len(c.Services) != 2 {
		t.Fatalf("services: got %d, want 2", len(c.Services))
	}
	svc := c.Services[0]
	if svc.Name != "UserService" {
		t.Fatalf("service name: got %q", svc.Name)
	}
	if len(svc.RPCs) != 2 {
		t.Fatalf("RPCs: got %d, want 2", len(svc.RPCs))
	}
	if svc.RPCs[0].Name != "GetUser" || svc.RPCs[0].Input != "GetUserRequest" || svc.RPCs[0].Output != "GetUserResponse" {
		t.Fatalf("RPC[0]: %+v", svc.RPCs[0])
	}
}

func TestParse_Messages(t *testing.T) {
	c := proto.Parse(fixture)
	if len(c.Messages) != 2 {
		t.Fatalf("messages: got %d, want 2", len(c.Messages))
	}
	msg := c.Messages[0]
	if msg.Name != "GetUserRequest" {
		t.Fatalf("message name: got %q", msg.Name)
	}
	if len(msg.Fields) != 2 {
		t.Fatalf("fields: got %d, want 2", len(msg.Fields))
	}
	if msg.Fields[0].Name != "user_id" || msg.Fields[0].Type != "string" || msg.Fields[0].Number != 1 {
		t.Fatalf("field[0]: %+v", msg.Fields[0])
	}
}

func TestConformance_Issues(t *testing.T) {
	c := proto.Parse(fixture)
	issues := proto.Conformance(c)

	// We expect:
	// 1. bad_service → service-pascal-case
	// 2. bad_service.get_user → rpc-pascal-case
	// 3. getUserResponse → message-pascal-case
	// 4. getUserResponse.UserName → field-snake-case
	wantRules := map[string]bool{
		"service-pascal-case": false,
		"rpc-pascal-case":     false,
		"message-pascal-case": false,
		"field-snake-case":    false,
	}
	for _, iss := range issues {
		wantRules[iss.Rule] = true
	}
	for rule, found := range wantRules {
		if !found {
			t.Errorf("expected rule %q to fire, issues: %v", rule, issues)
		}
	}
}

func TestConformance_Clean(t *testing.T) {
	clean := `
syntax = "proto3";
package foo.v1;
service FooService {
  rpc DoThing (DoThingRequest) returns (DoThingResponse);
}
message DoThingRequest {
  string request_id = 1;
}
message DoThingResponse {
  bool success = 1;
}
`
	c := proto.Parse(clean)
	issues := proto.Conformance(c)
	if len(issues) != 0 {
		t.Fatalf("expected no issues for clean proto, got: %v", issues)
	}
}

func TestConformance_Deterministic(t *testing.T) {
	c := proto.Parse(fixture)
	a := proto.Conformance(c)
	b := proto.Conformance(c)
	if len(a) != len(b) {
		t.Fatalf("non-deterministic length: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("non-deterministic at index %d: %+v vs %+v", i, a[i], b[i])
		}
	}
}
