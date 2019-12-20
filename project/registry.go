package project

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/kataras/iris-cli/utils"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v2"
)

const DefaultRegistryEndpoint = "https://iris-go.com/cli/registry.json"

type Registry struct {
	Endpoint      string                       `json:"endpoint,omitempty" yaml:"Endpoint" toml:"Endpoint"`
	EndpointAsset func(string) ([]byte, error) `json:"-" yaml:"-" toml:"-"` // If EndpointAsset is not nil then it reads the Endpoint from that `EndpointAsset` function.
	Projects      []*Project                   `json:"projects" yaml:"Projects" toml:"Projects"`
}

func NewRegistry() *Registry {
	return &Registry{
		Endpoint: DefaultRegistryEndpoint,
	}
}

func (r *Registry) Load() error {
	var (
		body []byte
		err  error
	)

	if r.EndpointAsset != nil {
		body, err = r.EndpointAsset(r.Endpoint)
		if err != nil {
			return err
		}
	} else {
		body, err = utils.Download(r.Endpoint, nil)
		if err != nil {
			// check if Endpoint is a file, and if it's read from it instead.
			if _, ioErr := os.Stat(r.Endpoint); ioErr == nil {
				body, err = ioutil.ReadFile(r.Endpoint)
				if err != nil {
					return err
				}
			} else if !os.IsNotExist(ioErr) { // if file probably exists but OS error.
				return fmt.Errorf("%w\n%w", err, ioErr)
			} else {
				return err // if download failed and file does not exist.
			}
		}
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
