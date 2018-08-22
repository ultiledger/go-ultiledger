package api

import (
	"log"
	"net"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	c "github.com/ultiledger/go-ultiledger/consensus"
	"github.com/ultiledger/go-ultiledger/ledger"
	"github.com/ultiledger/go-ultiledger/peer"
	pb "github.com/ultiledger/go-ultiledger/ultpb"
	"github.com/ultiledger/go-ultiledger/ultpb/rpc"
)

// Node is the central controller for ultiledger
type Node struct {
	// IP address of this node
	IP string
	// NodeID of this node
	NodeID string
	// start time of the node
	StartTime int64

	// config viper
	config *Config
	// zap logger
	logger *zap.SugaredLogger
	// ULTNode server
	server *NodeServer
	// peer manager
	pm *peer.Manager
	// ledger manager
	lm *ledger.Manager

	// engine of consensus
	engine *c.Engine

	// channel for receiving nomination
	nominateChan chan *pb.Statement
	// channel for receiving transaction
	txChan chan *pb.Tx
	// channel for stopping all the subroutines
	stopChan chan struct{}
}

// NewNode creates a Node which controls all the sub tasks
func NewNode(conf *Config) *Node {
	// initialize logger
	l, err := zap.NewProduction()
	if err != nil {
		log.Fatal(err)
	}

	// get outbound IP
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	addr := conn.LocalAddr().(*net.UDPAddr)

	ip := addr.String()
	nodeID := conf.NodeID

	txC := make(chan *pb.Tx)
	nominateC := make(chan *pb.Statement)

	node := &Node{
		config:       conf,
		logger:       l.Sugar(),
		server:       NewNodeServer(ip, nodeID, txC, nominateC),
		pm:           peer.NewManager(l.Sugar(), conf.Peers, ip, nodeID),
		IP:           ip,
		NodeID:       nodeID,
		StartTime:    time.Now().Unix(),
		txChan:       txC,
		nominateChan: nominateC,
		stopChan:     make(chan struct{}),
	}

	return node
}

// Start checks the provided configurations, if the config is valid,
// it will trigger sub goroutines to do the sub tasks.
func (u *Node) Start() error {
	// start node server
	go u.serveNode()

	// start node event loop
	go u.eventLoop()

	// start peer manager
	u.pm.Start(u.stopChan)

	select {}
	return nil
}

// Restart checks the provided configurations, if the config is valid,
// it will trigger sub goroutines to do the sub tasks.
func (u *Node) Restart() error {
	log.Println("restart called")
	return nil
}

// event loop for dealing with various internal events
func (u *Node) eventLoop() {
	for {
		select {
		case <-u.nominateChan:
			// TODO
		case <-u.txChan:
			// TODO
		case <-u.stopChan:
			u.logger.Infof("shutdown event loop")
			return
		}
	}
}

// serve starts a listener on the port and starts to accept request
func (u *Node) serveNode() {
	// register rpc service and start the ULTNode server
	listener, err := net.Listen("tcp", u.config.Port)
	if err != nil {
		u.logger.Fatal(err)
	}

	s := grpc.NewServer()
	rpc.RegisterNodeServer(s, u.server)

	u.logger.Infof("start to serve gRPC requests on %s", u.config.Port)
	go s.Serve(listener)

	for {
		select {
		case <-u.stopChan:
			u.logger.Infof("gracefully shutdown gRPC server")
			s.GracefulStop()
			return
		}
	}
}
