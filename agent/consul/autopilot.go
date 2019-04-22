package consul

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/consul/autopilot"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

// AutopilotDelegate is a Consul delegate for autopilot operations.
type AutopilotDelegate struct {
	server *Server
}

func (d *AutopilotDelegate) AutopilotConfig() *autopilot.Config {
	return d.server.getOrCreateAutopilotConfig()
}

func (d *AutopilotDelegate) FetchStats(ctx context.Context, servers []serf.Member) map[string]*autopilot.ServerStats {
	return d.server.statsFetcher.Fetch(ctx, servers)
}

func (d *AutopilotDelegate) IsServer(m serf.Member) (*autopilot.ServerInfo, error) {
	if m.Tags["role"] != "consul" {
		return nil, nil
	}

	portStr := m.Tags["port"]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}

	buildVersion, err := metadata.Build(&m)
	if err != nil {
		return nil, err
	}

	server := &autopilot.ServerInfo{
		Name:   m.Name,
		ID:     m.Tags["id"],
		Addr:   &net.TCPAddr{IP: m.Addr, Port: port},
		Build:  *buildVersion,
		Status: m.Status,
	}
	return server, nil
}

// Heartbeat a metric for monitoring if we're the leader
func (d *AutopilotDelegate) NotifyHealth(health autopilot.OperatorHealthReply) {
	if d.server.raft.State() == raft.Leader {
		metrics.SetGauge([]string{"autopilot", "failure_tolerance"}, float32(health.FailureTolerance))
		if health.Healthy {
			metrics.SetGauge([]string{"autopilot", "healthy"}, 1)
		} else {
			metrics.SetGauge([]string{"autopilot", "healthy"}, 0)
		}
	}
}

func (d *AutopilotDelegate) PromoteNonVoters(conf *autopilot.Config, health autopilot.OperatorHealthReply) ([]raft.Server, error) {
	future := d.server.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return nil, fmt.Errorf("failed to get raft configuration: %v", err)
	}
	servers := future.Configuration().Servers

	// Find any non-voters eligible for promotion.
	stable := autopilot.PromoteStableServers(conf, health, servers)
	// if no servers to add just return now
	if len(stable) == 0 {
		return stable, nil
	}

	// To avoid cycling over the members several times we create a map of them
	serverMembers := make(map[raft.ServerID]serf.Member)
	for _, member := range d.Serf().Members() {
		if member.Tags["role"] == "consul" { // Just include the servers
			serverMembers[raft.ServerID(member.Tags["id"])] = member
		}
	}

	// Remove non voting servers
	promoted := filterNonVoting(serverMembers, stable)
	// if no servers to add just return now
	if len(promoted) == 0 {
		return promoted, nil
	}

	// Filter by zone
	if conf.RedundancyZoneTag != "" {
		promoted = filterZoneServers(conf, serverMembers, promoted, servers)
	}
	return promoted, nil
}

func (d *AutopilotDelegate) Raft() *raft.Raft {
	return d.server.raft
}

func (d *AutopilotDelegate) Serf() *serf.Serf {
	return d.server.serfLAN
}

func filterNonVoting(serverMembers map[raft.ServerID]serf.Member, stable []raft.Server) []raft.Server {
	var promoted []raft.Server
	for _, server := range stable {
		member, ok := serverMembers[server.ID]
		if ok && member.Tags["nonvoter"] != "1" { // we add those that don't have the nonvoter tag
			promoted = append(promoted, server)
		}
	}
	return promoted
}

func filterZoneServers(conf *autopilot.Config, serverMembers map[raft.ServerID]serf.Member, initial []raft.Server, servers []raft.Server) []raft.Server {
	zoneVoter := make(map[string]bool)
	for _, server := range servers { // we set if there're a voter en every zone we know
		if member, ok := serverMembers[server.ID]; ok {
			zone := member.Tags[conf.RedundancyZoneTag]
			zoneVoter[zone] = zoneVoter[zone] || (autopilot.IsPotentialVoter(server.Suffrage) && member.Status != serf.StatusFailed)
		}
	}
	promoted := make([]raft.Server, 0)
	for _, server := range initial {
		zone := ""
		if member, ok := serverMembers[server.ID]; ok {
			zone = member.Tags[conf.RedundancyZoneTag]
		}
		if zone == "" || !zoneVoter[zone] { // If no zone or zone doesn't have a server
			promoted = append(promoted, server)
			zoneVoter[zone] = true // we set that we already got a server on that zone
		}
	}
	return promoted
}
