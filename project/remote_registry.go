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
	Endpoint string `json:"endpoint" yaml:"Endpoint" toml:"Endpoint"`
	// TODO: Find the structure of the registry file, i.e Repositories: or Projects: []Project and read from.
}

func NewRegistry() *Registry {
	return &Registry{
		Endpoint: DefaultRegistryEndpoint,
	}
}

func (r *Registry) Load() error {
	body, err := utils.Download(r.Endpoint, nil)
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
