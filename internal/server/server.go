package server

import (
	"appmsgbroker/event"
	"appmsgbroker/internal/state"
	"appmsgbroker/pkg/appmessage"
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"
)

type (
	Server struct {
		appmessage.UnimplementedAppMessageServiceServer
		mu    sync.Mutex
		wg    sync.WaitGroup
		State *state.ClusterState

		Leader   *LeaderState
		SelfId   string
		Watchers map[string]*Watcher
	}
)

// GetClusterState implements appmessage.AppMessageServiceServer.
func (s *Server) GetClusterState(context.Context, *appmessage.ClusterDiscoveryRequest) (*appmessage.ClusterDiscoveryResponse, error) {
	resp := s.State.ToDiscoveryResponse()

	resp.ReportingNode = &appmessage.ClusterNode{
		Id:       s.SelfId,
		IsLeader: s.Leader.NodeId == s.SelfId,
	}

	log.Println("return state>>", resp)

	return resp, nil
}

func (s *Server) RegisterNode(ctx context.Context, req *appmessage.RegisterNodeRequest) (*appmessage.RegisterNodeResponse, error) {
	log.Println("register node request...", req)
	s.mu.Lock()
	defer s.mu.Unlock()

	s.State.AddNode(&state.ClusterNode{
		Id:        req.Node.Id,
		StartedAt: req.Node.StartedAtEpoch,
		IsLeader:  req.Node.IsLeader,
		Address:   req.Node.Address,
	})

	log.Println("fire off watchers")
	for _, w := range s.Watchers {
		w.Complete()
	}

	return &appmessage.RegisterNodeResponse{}, nil
}

func (s *Server) StreamClusterState(stream appmessage.AppMessageService_StreamClusterStateServer) error {
	log.Println("client stream initiated...")
	nonce := 0
	var watcherId string
	var nodeId string

	send := func() error {
		nonce++
		resp := s.State.ToDiscoveryResponse()
		resp.ResponseNonce = fmt.Sprint(nonce)
		err := stream.Send(resp)
		if err != nil {
			log.Println("failed sending to client.. ", nodeId, err)
			return err
		}

		return nil
	}

	for {
		select {
		case <-stream.Context().Done():
			log.Println("client stream went away!", nodeId)
			return nil
		default:
			req, err := stream.Recv()
			if err == io.EOF {
				log.Println("client stream closed!", nodeId)
				return nil
			}
			if err != nil {
				log.Println("got stream error", nodeId)
				return err
			}

			log.Println("got streaming request", req)
			nodeId = req.NodeId

			// initial call...
			if req.ResponseNonce == "" || req.ResponseNonce == "0" {
				send()
			} else {
				watcherId = event.GenerateULID()
				ch := make(chan struct{})
				s.Watchers[watcherId] = &Watcher{
					signal: ch,
				}
				<-ch
				send()
				log.Println("watcher fulfilled...")
				s.mu.Lock()
				delete(s.Watchers, watcherId)
				s.mu.Unlock()
			}
		}

	}

}

func (s *Server) Init(ctx context.Context, id, addr string) error {
	self := &state.ClusterNode{
		Id:        id,
		StartedAt: time.Now().UTC().Unix(),
		Address:   addr,
	}

	// todo: return state?
	s.Leader.ResolveLeader(ctx, id)
	// am I the leader?
	if s.Leader.NodeId == id {
		self.IsLeader = true
		s.State.AddNode(self)
		return nil
	}

	stream, err := s.Leader.Client.StreamClusterState(ctx)
	if err != nil {
		return err
	}

	s.wg.Add(1)
	go func() {
		defer func() {
			log.Println("goroutine ended")
			log.Println(stream.CloseSend())
			s.wg.Done()
		}()

		for {
			select {

			// case <-ctx.Done():
			// 	log.Println("main context done!, close connection")

			// 	return
			case <-stream.Context().Done():
				log.Println("client closed")

				return
			case <-time.After(5 * time.Second):
				log.Println("close out after timeout")
				return
				// default:
				// 	in, err := stream.Recv()
				// 	if err == io.EOF {
				// 		// read done.
				// 		log.Println("leader closed the connection")
				// 		return
				// 	}
				// 	if err != nil {
				// 		log.Printf("leader stream failed: %v", err)
				// 		return
				// 	}
				// 	log.Printf("Got message back! update state to: %v", in)
				// 	s.mu.Lock()
				// 	s.State = state.NewFromResponse(in)
				// 	s.mu.Unlock()

				// 	log.Println("send another request ...")
				// 	if err := stream.Send(&appmessage.ClusterDiscoveryRequest{
				// 		VersionInfo:   in.VersionInfo,
				// 		NodeId:        s.SelfId,
				// 		ResponseNonce: in.ResponseNonce,
				// 	}); err != nil {
				// 		log.Printf("send new request failed: %v", err)
				// 		return
				// 	}
			}

		}
	}()

	err = stream.Send(&appmessage.ClusterDiscoveryRequest{
		VersionInfo: fmt.Sprint(s.State.Version),
		NodeId:      s.SelfId,
	})
	if err != nil {
		s.wg.Done()
		return err
	}

	// notify leader I exist
	_, err = s.Leader.Client.RegisterNode(ctx, &appmessage.RegisterNodeRequest{
		Node: &appmessage.ClusterNode{
			Id:             s.SelfId,
			StartedAtEpoch: self.StartedAt,
			Address:        self.Address,
		},
	})
	if err != nil {
		s.wg.Done()
	}
	return err
}

func (s *Server) Stop() {
	s.wg.Wait()
}

// func (s *Server) RegisterNewNode(ctx context.Context, node *Node) error {
// 	log.Println("register node...", node)
// 	leader := s.findTheLeader(ctx)

// 	if s.Leader && s.SelfId == node.Id {
// 		node.Master = true
// 	}

// 	if leader == nil {
// 		s.State = &ClusterState{
// 			Version: s.State.Version + 1,
// 			Nodes: map[string]*Node{
// 				node.Id: node,
// 			},
// 		}

// 		// TODO: emit
// 		return nil
// 	}

// 	log.Println("TODO: register with master...")

// 	cons := make([]*appmessage.MessageConsumer, 0)
// 	_, err := s.leaderClient(ctx).RegisterNode(ctx, &appmessage.RegisterNodeRequest{
// 		Node: &appmessage.ClusterNode{
// 			Id:             node.Id,
// 			StartedAtEpoch: node.StartedAt,
// 			Address:        node.Address,
// 			IsMaster:       node.Master,
// 			Consumers:      cons,
// 		},
// 	})
// 	if err != nil {
// 		log.Fatalf("could not register node! %v", err)
// 	}

// 	return nil
// }

// func (s *Server) EstablishLeader(ctx context.Context) error {
// 	leader := s.findTheLeader(ctx)
// 	if leader == nil {
// 		s.Leader = true
// 		return nil
// 	}

// 	fmt.Println("connect to leader", leader.Address)

// 	conn, err := grpc.Dial(leader.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
// 	if err != nil {
// 		return err
// 	}

// 	client := appmessage.NewAppMessageServiceClient(conn)

// 	stream, err := client.StreamClusterState(ctx)
// 	if err != nil {
// 		return err
// 	}

// 	go func() {
// 		for {
// 			in, err := stream.Recv()
// 			if err == io.EOF {
// 				// read done.
// 				log.Println("leader closed the connection")
// 				return
// 			}
// 			if err != nil {
// 				log.Fatalf("leader streamfailed: %v", err)
// 			}
// 			log.Printf("Got message back!%v", in)
// 			log.Println("send another...")
// 			if err := stream.Send(&appmessage.ClusterDiscoveryRequest{
// 				VersionInfo:   in.VersionInfo,
// 				NodeId:        s.SelfId,
// 				ResponseNonce: in.ResponseNonce,
// 			}); err != nil {
// 				log.Fatalf("send new request failed: %v", err)
// 			}
// 		}
// 	}()

// 	err = stream.Send(&appmessage.ClusterDiscoveryRequest{
// 		VersionInfo: fmt.Sprint(s.State.Version),
// 		NodeId:      s.SelfId,
// 	})
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

// func (s *Server) findTheLeader(ctx context.Context) *Node {
// 	if s.Leader {
// 		log.Println("I'm already the leader!")
// 		return nil
// 	}

// 	if len(s.State.Nodes) == 0 {
// 		log.Println("I'm the leader!")
// 		return nil
// 	}

// 	for _, n := range s.State.Nodes {
// 		if n.Master {
// 			log.Println("I'm not the leader :(")
// 			return n
// 		}
// 	}

// 	// TODO: how to determine leader
// 	panic("i don't know yet how to follow the leader")
// 	return nil
// }

// func (s *Server) leaderClient(ctx context.Context) appmessage.AppMessageServiceClient {
// 	leader := s.findTheLeader(ctx)
// 	if leader == nil {
// 		panic("I don't have a leader to follow!!")
// 	}

// 	fmt.Println("connect to leader", leader.Address)

// 	conn, err := grpc.Dial(leader.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
// 	if err != nil {
// 		log.Fatalf("fail to dial: %v, skipping...", err)
// 	}

// 	return appmessage.NewAppMessageServiceClient(conn)
// }

var _ appmessage.AppMessageServiceServer = &Server{}
