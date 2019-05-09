// +build !consulent

package consul

import (
	"fmt"
	"net"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/serf/serf"
)

// TODO-MEIGAS: New API operator/segment/list || and new command: segment list

const serfSegmentSnapshot = "serf/%s.snapshot"

// LANMembersAllSegments returns members from all segments.
func (s *Server) LANMembersAllSegments() ([]serf.Member, error) {
	members := make([]serf.Member, 0)
	members = append(members, s.LANMembers()...)
	for _, s := range s.segmentLAN {
		for _, m := range s.Members() {
			if ok, _ := isConsulNode(m); ok {
				members = append(members, m)
			}
		}
	}

	return members, nil
}

// LANSegmentMembers is used to return the members of the given LAN segment.
func (s *Server) LANSegmentMembers(segment string) ([]serf.Member, error) {
	if segment == "" {
		return s.LANMembers(), nil
	}
	serfSegment, ok := s.segmentLAN[segment]
	if ok {
		return serfSegment.Members(), nil
	}

	return nil, structs.ErrSegmentsNotSupported
}

// LANSegmentAddr is used to return the address used for the given LAN segment.
func (s *Server) LANSegmentAddr(name string) string {
	return ""
}

// setupSegmentRPC returns an error if any segments are defined since the OSS
// version of Consul doesn't support them.
func (s *Server) setupSegmentRPC() (map[string]net.Listener, error) {
	// if len(s.config.Segments) > 0 {
	// 	return nil, structs.ErrSegmentsNotSupported
	// }

	return nil, nil
}

// setupSegments returns an error if any segments are defined since the OSS
// version of Consul doesn't support them.
func (s *Server) setupSegments(config *Config, port int, rpcListeners map[string]net.Listener) error {
	// if len(config.Segments) > 0 {
	// 	return structs.ErrSegmentsNotSupported
	// }
	for _, segment := range config.Segments {
		// I think there's no need to create a new Channel to get events
		segmentSerf, err := s.setupSerf(segment.SerfConfig, nil, fmt.Sprintf(serfSegmentSnapshot, segment.Name), false, port, segment.Name, s.Listener)
		if err != nil {
			return err
		}
		s.segmentLAN[segment.Name] = segmentSerf
	}

	return nil
}

// floodSegments is a NOP in the OSS version of Consul.
func (s *Server) floodSegments(config *Config) {
	for name, segment := range s.segmentLAN {
		addrFn := func(s *metadata.Server) (string, bool) {
			v, ok := s.SegmentAddrs[name]
			return v, ok
		}
		portFn := func(s *metadata.Server) (int, bool) {
			v, ok := s.SegmentPorts[name]
			return v, ok
		}
		go s.Flood(addrFn, portFn, segment)
	}
}

// reconcile is used to reconcile the differences between Serf membership and
// what is reflected in our strongly consistent store. Mainly we need to ensure
// all live nodes are registered, all failed nodes are marked as such, and all
// left nodes are de-registered.
func (s *Server) reconcile() (err error) {
	defer metrics.MeasureSince([]string{"leader", "reconcile"}, time.Now())
	members, _ := s.LANMembersAllSegments()
	knownMembers := make(map[string]struct{})
	for _, member := range members {
		if err := s.reconcileMember(member); err != nil {
			return err
		}
		knownMembers[member.Name] = struct{}{}
	}

	// Reconcile any members that have been reaped while we were not the
	// leader.
	return s.reconcileReaped(knownMembers)
}
