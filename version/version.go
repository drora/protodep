package version

import (
	"fmt"
)

const (
	fork    = "github.com/drora/protodep"
	version = "0.9.6"
)

type Info struct {
	Fork    string
	Version string
}

func Get() Info {
	return Info{
		Fork:    fork,
		Version: version,
	}
}

func (i Info) String() string {
	return fmt.Sprintf(`{"Fork": "%s", "Version": "%s"}`, i.Fork, i.Version)
}
