package api

import (
	"net"
	"time"

	"google.golang.org/grpc"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/consensus"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/future"
	"github.com/ultiledger/go-ultiledger/ledger"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/peer"
	"github.com/ultiledger/go-ultiledger/rpc"
	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
)

// Node is the central controller for ultiledger
type Node struct {
	store db.DB

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
	txFuture     chan *future.Tx
	peerFuture   chan *future.Peer
	stmtFuture   chan *future.Statement
	txsetFuture  chan *future.TxSet
	quorumFuture chan *future.Quorum
}

// NewNode creates a Node which controls all the sub tasks
func NewNode(conf *Config) *Node {
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

	pm := peer.NewManager(conf.Peers, addr, nodeID)
	lm := ledger.NewManager(store)
	am := account.NewManager(store)

	// construct consensus engine context and create consensus engine
	engineCtx := &consensus.EngineContext{
		Store:  store,
		Seed:   seed,
		NodeID: nodeID,
		PM:     pm,
		AM:     am,
		LM:     lm,
		Quorum: conf.Quorum,
	}
	engine := consensus.NewEngine(engineCtx)

	txFuture := make(chan *future.Tx)
	peerFuture := make(chan *future.Peer)
	stmtFuture := make(chan *future.Statement)

	// construct node server context and create node server
	serverCtx := &rpc.ServerContext{
		Addr:   addr,
		NodeID: nodeID,
		Seed:   seed,
		PF:     peerFuture,
		TF:     txFuture,
		SF:     stmtFuture,
	}
	nodeServer := rpc.NewNodeServer(serverCtx)

	// create local node
	node := &Node{
		config:    conf,
		store:     store,
		server:    nodeServer,
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
func (n *Node) Start() error {
	// start node server
	go n.serveNode()

	// start node event loop
	go n.eventLoop()

	// start peer manager
	n.pm.Start(n.stopChan)

	select {}
	return nil
}

// Restart checks the provided configurations, if the config is valid,
// it will trigger sub goroutines to do the sub tasks.
func (n *Node) Restart() error {
	return nil
}

// Event loop for processing server received messages
func (n *Node) eventLoop() {
	for {
		select {
		case txf := <-n.txFuture:
			err := n.engine.RecvTx(txf.Tx)
			if err != nil {
				log.Errorf("add tx failed: %v", err)
			}
			txf.Respond(err)
		case pf := <-n.peerFuture:
			err := n.pm.AddPeerAddr(pf.Addr)
			if err != nil {
				log.Errorf("add peer addr failed: %v", err)
			}
			pf.Respond(err)
		case sf := <-n.stmtFuture:
			err := n.engine.RecvStatement(sf.Stmt)
			if err != nil {
				log.Errorf("recv statement failed: %v", err)
			}
			sf.Respond(err)
		case tsf := <-n.txsetFuture:
			err := n.engine.RecvTxSet(tsf.TxSetHash, tsf.TxSet)
			if err != nil {
				log.Errorf("recv txset failed: %v", err)
			}
			tsf.Respond(err)
		case qf := <-n.quorumFuture:
			err := n.engine.RecvQuorum(qf.QuorumHash, qf.Quorum)
			if err != nil {
				log.Errorf("redv qf failed: %v", err)
			}
			qf.Respond(err)
		case <-n.stopChan:
			log.Info("shutdown event loop")
			return
		}
	}
}

// serve starts a listener on the port and starts to accept request
func (n *Node) serveNode() {
	// register rpc service and start the ULTNode server
	listener, err := net.Listen("tcp", n.config.Port)
	if err != nil {
		log.Fatal(err)
	}

	s := grpc.NewServer()
	rpcpb.RegisterNodeServer(s, n.server)

	log.Infof("start to serve gRPC server on %s", n.config.Port)
	go s.Serve(listener)

	for {
		select {
		case <-n.stopChan:
			log.Infof("gracefully shutdown gRPC server")
			s.GracefulStop()
			return
		}
	}
}
