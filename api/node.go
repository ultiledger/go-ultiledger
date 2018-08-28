package api

import (
	"log"
	"net"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/consensus"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/future"
	"github.com/ultiledger/go-ultiledger/ledger"
	"github.com/ultiledger/go-ultiledger/peer"
	"github.com/ultiledger/go-ultiledger/rpc"
	pb "github.com/ultiledger/go-ultiledger/rpc/rpcpb"
)

// Node is the central controller for ultiledger
type Node struct {
	store  db.DB
	logger *zap.SugaredLogger

	// Network address of this node
	addr string
	// NodeID and seed of this node
	nodeID string
	seed   string
	// start time of the node
	startTime int64

	config *Config

	server *rpc.NodeServer
	pm     *peer.Manager
	lm     *ledger.Manager
	am     *account.Manager
	engine *consensus.Engine

	// channel for stopping all the subroutines
	stopChan chan struct{}

	// futures for task with error responses
	txFuture chan *future.Tx
}

// NewNode creates a Node which controls all the sub tasks
func NewNode(conf *Config) *Node {
	// initialize logger
	l, err := zap.NewProduction()
	if err != nil {
		log.Fatal(err)
	}
	logger := l.Sugar()

	// get outbound IP
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	netAddr := conn.LocalAddr().(*net.UDPAddr)

	addr := netAddr.String() + ":" + conf.Port
	nodeID := conf.NodeID
	seed := conf.Seed

	// create database store
	ctor, err := db.GetDB(conf.DBBackend)
	if err != nil {
		log.Fatal(err)
	}
	store := ctor(conf.DBPath)

	pm := peer.NewManager(l.Sugar(), conf.Peers, addr, nodeID)
	lm := ledger.NewManager(store, logger)
	am := account.NewManager(store, logger)
	engine := consensus.NewEngine(store, logger, pm, am, lm)
	txFuture := make(chan *future.Tx)
	peerFuture := make(chan *future.Peer)

	node := &Node{
		config:    conf,
		store:     store,
		logger:    logger,
		server:    rpc.NewNodeServer(logger, addr, nodeID, seed, peerFuture, txFuture),
		pm:        pm,
		lm:        lm,
		am:        am,
		engine:    engine,
		addr:      addr,
		nodeID:    nodeID,
		startTime: time.Now().Unix(),
		stopChan:  make(chan struct{}),
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
	pb.RegisterNodeServer(s, u.server)

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
