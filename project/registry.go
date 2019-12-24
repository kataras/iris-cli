package project

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"sort"
	"strings"

	"github.com/kataras/iris-cli/utils"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v2"
)

const DefaultRegistryEndpoint = "https://iris-go.com/cli/registry.json"

type Registry struct {
	Endpoint      string                       `json:"endpoint,omitempty" yaml:"Endpoint" toml:"Endpoint"`
	EndpointAsset func(string) ([]byte, error) `json:"-" yaml:"-" toml:"-"`                      // If EndpointAsset is not nil then it reads the Endpoint from that `EndpointAsset` function.
	Projects      map[string]string            `json:"projects" yaml:"Projects" toml:"Projects"` // key = name, value = repo.
	installed     map[string]struct{}
	Names         []string `json:"-" yaml:"-" toml:"-"` // sorted Projects names.
}

func NewRegistry() *Registry {
	return &Registry{
		Endpoint:  DefaultRegistryEndpoint,
		Projects:  make(map[string]string),
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

	switch ext := utils.Ext(r.Endpoint); ext {
	case ".json":
		err = json.Unmarshal(body, r)
	case ".yaml", ".yml":
		err = yaml.Unmarshal(body, r)
	case ".toml", ".tml":
		err = toml.Unmarshal(body, r)
	default:
		err = fmt.Errorf("unknown extension: %s", ext)
	}

	if err != nil {
		return err
	}

	names := make([]string, 0, len(r.Projects))
	for name := range r.Projects {
		names = append(names, name)
	}
	sort.Strings(names)
	r.Names = names
	return nil
}

// ErrProjectNotExists can be return as error value from the `Registry.Install` method.
var ErrProjectNotExists = fmt.Errorf("project not exists")

// Exists reports whether a project with "name" exists in the registry.
func (r *Registry) Exists(name string) (string, bool) {
	repo, ok := r.Projects[name]
	return repo, ok
}

// Install downloads and unzips a project with "name" to "dest" as "module".
func (r *Registry) Install(p *Project) error {
	for projectName, repo := range r.Projects {
		if projectName != p.Name {
			continue
		}

		p.Repo = repo

		err := p.Install()
		if err == nil {
			r.installed[projectName] = struct{}{}
		}
		return err
	}

	return ErrProjectNotExists
}
