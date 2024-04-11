package server

import (
	"appmsgbroker/pkg/appmessage"
	"context"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type LeaderState struct {
	GetNodeAddrsFn func() []string
	NodeId         string
	Client         appmessage.AppMessageServiceClient
}

func (l *LeaderState) ResolveLeader(ctx context.Context, callingId string) error {
	log.Println("determine who is the leader...")
	addrs := l.GetNodeAddrsFn()
	if len(addrs) == 0 {
		log.Println("I am the leader, there are no others!")
		l.NodeId = callingId
	}

	// nag servers for information
	for _, addr := range addrs {
		conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("fail to dial: %v, skipping...", err)
			continue
		}
		log.Println("connected to: ", addr)
		client := appmessage.NewAppMessageServiceClient(conn)
		resp, err := client.GetClusterState(ctx, &appmessage.ClusterDiscoveryRequest{})
		if err != nil {
			log.Printf("failed to get response: %v, skipping...", err)
			continue
		}
		log.Println("got response!", resp)
		if resp.ReportingNode.IsLeader {
			log.Println("found the leader!", resp.ReportingNode)
			l.NodeId = resp.ReportingNode.Id
			l.Client = client
			return nil
		}
		conn.Close()
	}

	log.Println("I am the leader, no body else claimed it!")
	l.NodeId = callingId
	return nil
}
