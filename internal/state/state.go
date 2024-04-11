package state

import (
	"appmsgbroker/pkg/appmessage"
	"fmt"
	"strconv"
	"sync"
)

type (
	// ClusterConsumer struct {
	// 	Id                string
	// 	LocalConsumer     event.Consumer[any]
	// 	ActiveSubscribers int32
	// }

	ClusterNode struct {
		Id        string
		StartedAt int64
		IsLeader  bool
		// Consumers map[string]*ClusterConsumer
		Address string
	}

	ClusterState struct {
		mu      sync.Mutex
		Version int
		Nodes   map[string]*ClusterNode
	}
)

func New() *ClusterState {
	return &ClusterState{
		Nodes: make(map[string]*ClusterNode),
	}
}

func NewFromResponse(res *appmessage.ClusterDiscoveryResponse) *ClusterState {
	ver, _ := strconv.Atoi(res.VersionInfo)
	nodes := make(map[string]*ClusterNode)
	for _, n := range res.Nodes {
		nodes[n.Id] = &ClusterNode{
			Id:        n.Id,
			StartedAt: n.StartedAtEpoch,
			IsLeader:  n.IsLeader,
			Address:   n.Address,
		}
	}

	return &ClusterState{
		Version: ver,
		Nodes:   nodes,
	}
}

func (cs *ClusterState) ToDiscoveryResponse() *appmessage.ClusterDiscoveryResponse {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	nodes := make([]*appmessage.ClusterNode, 0)

	for _, n := range cs.Nodes {
		// consumers := make([]*appmessage.MessageConsumer, 0)

		nodes = append(nodes, &appmessage.ClusterNode{
			Id:             n.Id,
			StartedAtEpoch: n.StartedAt,
			IsLeader:       n.IsLeader,
			Address:        n.Address,
		})
	}

	return &appmessage.ClusterDiscoveryResponse{
		VersionInfo: fmt.Sprint(cs.Version),
		Nodes:       nodes,
	}
}

func (cs *ClusterState) AddNode(n *ClusterNode) *ClusterState {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.Nodes[n.Id] = n
	cs.Version++

	return cs
}

func (cs *ClusterState) RemoveNode(id string) *ClusterState {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	delete(cs.Nodes, id)
	cs.Version++

	return cs
}
