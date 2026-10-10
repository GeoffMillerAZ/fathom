package main

import (
	"reflect"
	"testing"
)

func TestConvert(t *testing.T) {
	dump := `{"contexts":{"application":{"mappings":{"dispatcherServlets":{"dispatcherServlet":[
	 {"handler":"x","details":{"handlerMethod":{"className":"org.p.owner.OwnerController","name":"show"},
	  "requestMappingConditions":{"methods":["GET"],"patterns":["/owners/{ownerId}"]}}},
	 {"handler":"y","details":{"handlerMethod":{"className":"org.p.Outer$Inner","name":"any"},
	  "requestMappingConditions":{"methods":[],"patterns":["/a","/b"]}}},
	 {"handler":"z","details":{"handlerMethod":{"className":"org.springframework.boot.webmvc.error.BasicErrorController","name":"error"},
	  "requestMappingConditions":{"methods":[],"patterns":["/error"]}}},
	 {"handler":"ResourceHttpRequestHandler [classpath [static/]]","predicate":"/**"}
	]}}}}}`
	got, err := convert([]byte(dump), "org.p")
	if err != nil {
		t.Fatal(err)
	}
	want := []route{
		{Method: "ANY", Path: "/a", Handler: "Outer.Inner.any"},
		{Method: "ANY", Path: "/b", Handler: "Outer.Inner.any"},
		{Method: "GET", Path: "/owners/{ownerId}", Handler: "OwnerController.show"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}
