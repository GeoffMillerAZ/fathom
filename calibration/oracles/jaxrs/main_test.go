package main

import (
	"reflect"
	"testing"
)

func TestConvert(t *testing.T) {
	doc := `{"openapi":"3.1.0","paths":{
	 "/api/articles":{"get":{"operationId":"ArticlesResource_list"},"post":{"operationId":"ArticlesResource_create"},"parameters":[]},
	 "/api/articles/{slug}":{"delete":{"operationId":"ArticlesResource_delete"}},
	 "/api":{"get":{}},
	 "/q/health":{"get":{"operationId":"Health_check"}}}}`
	got, err := convert([]byte(doc), "/api/")
	if err != nil {
		t.Fatal(err)
	}
	want := []route{
		{Method: "GET", Path: "/", Handler: ""},
		{Method: "GET", Path: "/articles", Handler: "ArticlesResource.list"},
		{Method: "POST", Path: "/articles", Handler: "ArticlesResource.create"},
		{Method: "DELETE", Path: "/articles/{slug}", Handler: "ArticlesResource.delete"},
		{Method: "GET", Path: "/q/health", Handler: "Health.check"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}
