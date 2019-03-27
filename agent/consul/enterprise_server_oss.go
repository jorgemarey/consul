// +build !ent

package consul

import (
	"fmt"
	"net"

	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/serf/serf"
)

type EnterpriseServer struct{}

func (s *Server) initEnterprise() error {
	return nil
}

func (s *Server) startEnterprise() error {
	return nil
}

func (s *Server) handleEnterpriseUserEvents(event serf.UserEvent) bool {
	return false
}

func (s *Server) handleEnterpriseRPCConn(rtype pool.RPCType, conn net.Conn, isTLS bool) bool {
	return false
}

func (s *Server) enterpriseStats() map[string]map[string]string {
	stats := map[string]map[string]string{}
	for k, v := range s.segmentLAN {
		stats[fmt.Sprintf("serf_segment_%s", k)] = v.Stats()
	}

	return stats
}

func (s *Server) intentionReplicationEnabled() bool {
	return false
}
