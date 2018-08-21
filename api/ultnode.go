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

// ultNode is the central controller for ultiledger
type ultNode struct {
	// IP address of this node
	IP string
	// NodeID of this node
	NodeID string
	// start time of the node
	StartTime int64

	// config viper
	config *ultNodeConfig
	// zap logger
	logger *zap.SugaredLogger
	// ULTNode server
	server *ULTNodeServer
	// peer manager
	pm *peer.PeerManager
	// ledger manager
	lm *ledger.LedgerManager

	// engine of consensus
	engine *c.Engine

	// channel for receiving nomination
	nominateChan chan *pb.Statement
	// channel for receiving transaction
	txChan chan *pb.Tx
	// channel for stopping all the subroutines
	stopChan chan struct{}
}

// NewULTNode creates a ultNode which controls all the sub tasks
func NewULTNode(conf *ultNodeConfig) *ultNode {
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

	node := &ultNode{
		config:       conf,
		logger:       l.Sugar(),
		server:       NewULTNodeServer(ip, nodeID, txC, nominateC),
		pm:           peer.NewPeerManager(l.Sugar(), conf.Peers, ip, nodeID),
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
func (u *ultNode) Start() error {
	// start node server
	go u.serveULTNode()

	// start node event loop
	go u.eventLoop()

	// start peer manager
	u.pm.Start(u.stopChan)

	select {}
	return nil
}

// Restart checks the provided configurations, if the config is valid,
// it will trigger sub goroutines to do the sub tasks.
func (u *ultNode) Restart() error {
	log.Println("restart called")
	return nil
}

// event loop for dealing with various internal events
func (u *ultNode) eventLoop() {
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
func (u *ultNode) serveULTNode() {
	// register rpc service and start the ULTNode server
	listener, err := net.Listen("tcp", u.config.Port)
	if err != nil {
		u.logger.Fatal(err)
	}

	s := grpc.NewServer()
	rpc.RegisterULTNodeServer(s, u.server)

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
