// +build !consulent

package consul

import (
	"github.com/hashicorp/consul/agent/consul/autopilot"
	improvedAutopilot "github.com/jorgemarey/autopilot"
)

func (s *Server) initAutopilot(config *Config) {
	apDelegate := improvedAutopilot.New(s.logger, &AutopilotDelegate{s})
	s.autopilot = autopilot.NewAutopilot(s.logger, apDelegate, config.AutopilotInterval, config.ServerHealthInterval)
}
