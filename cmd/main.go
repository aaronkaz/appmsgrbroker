package main

import (
	"appmsgbroker/internal/routine"
	"appmsgbroker/internal/server"
	"appmsgbroker/internal/state"
	"appmsgbroker/pkg/appmessage"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"

	"google.golang.org/grpc"
)

var (
	port  = flag.Int("port", 5000, "The server port")
	nodes = flag.String("nodes", "", "Static list of nodes to connect to")
)

const (
	exitCodeErr       = 1
	exitCodeInterrupt = 2
)

func main() {
	log.SetOutput(os.Stdout)
	flag.Parse()
	log.Println("starting app....")
	// log.Printf("envs -- MY_NODE_NAME = %s, MY_POD_NAME = %s, MY_POD_NAMESPACE = %s, MY_POD_IP = %s",
	// 	os.Getenv("MY_NODE_NAME"), os.Getenv("MY_POD_NAME"), os.Getenv("MY_POD_NAMESPACE"), os.Getenv("MY_POD_IP"))

	ctx := context.Background()
	// ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	// defer stop()

	ctx, cancel := context.WithCancel(ctx)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	defer func() {
		signal.Stop(signalChan)
		cancel()
	}()
	go func() {
		select {
		case <-signalChan: // first signal, cancel context
			log.Println("cancelling...")
			cancel()
		case <-ctx.Done():
			log.Println("already done...")
		}
		<-signalChan // second signal, hard exit
		log.Println("got second signal...")
		os.Exit(exitCodeInterrupt)
	}()

	if err := run(ctx); err != nil {
		log.Println("run error: ", err)
	}
}

func run(ctx context.Context) error {
	// clusterState := getClusterState(ctx)
	// log.Println("current state>>>", clusterState)

	selfId := fmt.Sprintf("node/test-app:%v", *port)
	addr := fmt.Sprintf("localhost:%v", *port)

	// self := &internal.Node{
	// 	Id:        selfId,
	// 	StartedAt: time.Now().UTC().Unix(),
	// 	Consumers: make(map[string]*internal.ClusterConsumer),
	// 	Address:   fmt.Sprintf("localhost:%v", *port),
	// }

	// log.Println("self is >>", self)

	srv := &server.Server{
		SelfId:   selfId,
		State:    state.New(),
		Watchers: make(map[string]*server.Watcher),
		Leader: &server.LeaderState{
			GetNodeAddrsFn: func() []string {
				addrs := strings.Split(*nodes, ",")
				if *nodes == "" || len(addrs) == 0 {
					return []string{}
				}

				return addrs
			},
		},
	}

	if err := srv.Init(ctx, selfId, addr); err != nil {
		return fmt.Errorf("failed to initialize server: %v", err)
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	appmessage.RegisterAppMessageServiceServer(grpcServer, srv)

	log.Println("starting grpc server... on port: ", *port)

	return routine.Run(ctx, func(ctx context.Context) error {
		return grpcServer.Serve(lis)
	}, func() {
		log.Println("wait for server to stop")
		srv.Stop()
		log.Println("Stopping server")
		lis.Close()
	})
}

// func getClusterState(ctx context.Context) *internal.ClusterState {
// 	addrs := strings.Split(*nodes, ",")
// 	if *nodes == "" || len(addrs) == 0 {
// 		return &internal.ClusterState{
// 			Nodes: make(map[string]*internal.Node),
// 		}
// 	}

// 	// nag servers for information
// 	for _, addr := range addrs {
// 		conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
// 		if err != nil {
// 			log.Printf("fail to dial: %v, skipping...", err)
// 			continue
// 		}
// 		defer conn.Close()
// 		log.Println("connected to: ", addr)
// 		client := appmessage.NewAppMessageServiceClient(conn)
// 		resp, err := client.GetClusterState(ctx, &appmessage.ClusterDiscoveryRequest{})
// 		if err != nil {
// 			log.Printf("failed to get response: %v, skipping...", err)
// 			continue
// 		}
// 		log.Println("got response!", resp)
// 		return responseToState(resp)
// 	}

// 	return &internal.ClusterState{
// 		Nodes: make(map[string]*internal.Node),
// 	}
// }

// func responseToState(res *appmessage.ClusterDiscoveryResponse) *internal.ClusterState {
// 	ver, _ := strconv.Atoi(res.VersionInfo)
// 	state := &internal.ClusterState{
// 		Version: ver,
// 		Nodes:   make(map[string]*internal.Node),
// 	}

// 	for _, n := range res.Nodes {
// 		sn := &internal.Node{
// 			Id:        n.Id,
// 			StartedAt: n.StartedAtEpoch,
// 			Master:    n.IsMaster,
// 			Consumers: make(map[string]*internal.ClusterConsumer),
// 			Address:   n.Address,
// 		}
// 		for _, c := range n.Consumers {
// 			sn.Consumers[n.Id] = &internal.ClusterConsumer{
// 				Id:                n.Id,
// 				ActiveSubscribers: c.ActiveSubscribers,
// 			}
// 		}
// 		state.Nodes[n.Id] = sn
// 	}

// 	return state
// }

// func registerState(leader *appmessage.ClusterNode, self *internal.Node) *internal.ClusterState
