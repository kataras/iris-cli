package project

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"

	"github.com/kataras/iris-cli/utils"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v2"
)

const DefaultRegistryEndpoint = "https://iris-go.com/cli/registry.json"

type Registry struct {
	Endpoint      string                       `json:"endpoint,omitempty" yaml:"Endpoint" toml:"Endpoint"`
	EndpointAsset func(string) ([]byte, error) `json:"-" yaml:"-" toml:"-"` // If EndpointAsset is not nil then it reads the Endpoint from that `EndpointAsset` function.
	Projects      map[string]*Project          `json:"projects" yaml:"Projects" toml:"Projects"`
	installed     map[string]struct{}
}

func NewRegistry() *Registry {
	return &Registry{
		Endpoint:  DefaultRegistryEndpoint,
		Projects:  make(map[string]*Project),
		installed: make(map[string]struct{}),
	}
}

func (r *Registry) Load() error {
	var (
		body []byte
		err  error
	)

	if r.EndpointAsset != nil {
		body, err = r.EndpointAsset(r.Endpoint)
	} else {
		if isURL := strings.HasPrefix(r.Endpoint, "http"); isURL {
			if _, urlErr := url.Parse(r.Endpoint); urlErr != nil {
				return err
			}
			body, err = utils.Download(r.Endpoint, nil)
		} else {
			body, err = ioutil.ReadFile(r.Endpoint)
		}
	}

	if err != nil {
		return err
	}

	ext := ".json"
	if extIdx := strings.LastIndexByte(r.Endpoint, '.'); extIdx > 0 {
		ext = r.Endpoint[extIdx:]
	}

	switch ext {
	case ".json":
		return json.Unmarshal(body, r)
	case ".yaml", ".yml":
		return yaml.Unmarshal(body, r)
	case ".toml", ".tml":
		return toml.Unmarshal(body, r)
	default:
		return fmt.Errorf("unknown extension: %s", ext)
	}
}

// ErrProjectNotExists can be return as error value from the `Registry.Install` method.
var ErrProjectNotExists = fmt.Errorf("project not exists")

// Exists reports whether a project with "name" exists in the registry.
func (r *Registry) Exists(name string) bool {
	_, ok := r.Projects[name]
	return ok
}

// Install downloads and unzips a project with "name" to "dest" as "module".
func (r *Registry) Install(name string, module, dest string) error {
	if p, ok := r.Projects[name]; ok {
		// we use pointers, so this will be saved, as we want to - in the future we will have a list projects command too.
		p.Module = module
		p.Dest = dest
		err := p.Install()
		if err == nil {
			r.installed[name] = struct{}{}
		}
		return err
	}

	return ErrProjectNotExists
}
