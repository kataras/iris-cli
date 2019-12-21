package project

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestRemoteRegistryLoad(t *testing.T) {
	var (
		expected = &Registry{Projects: []*Project{
			{Repo: "github.com/kataras/iris"},
			{Repo: "github.com/kataras/neffos"},
			{Repo: "github.com/kataras/neffos.js"},
		}}

		tests = []func(*Registry) *Registry{
			newTestRegistryEndpointAsset,
		}
	)

	for _, tt := range tests {
		reg := tt(expected)
		if err := reg.Load(); err != nil {
			t.Fatal(err)
		}

		if expected, got := len(expected.Projects), len(reg.Projects); expected != got {
			t.Fatalf("expected length of projects: %d but got %d", expected, got)
		}

		for i := range reg.Projects {
			if expected, got := expected.Projects[i], reg.Projects[i]; !reflect.DeepEqual(expected, got) {
				t.Fatalf("project failed to load: expected:\n%#+v\nbut got\n%#+v", expected, got)
			}
		}
	}
}

func newTestRegistryEndpointAsset(expectedProjects *Registry) *Registry {
	reg := NewRegistry()
	reg.Endpoint = "./test.json"
	reg.EndpointAsset = func(string) ([]byte, error) {
		return json.Marshal(expectedProjects)
	}
	return reg
}
