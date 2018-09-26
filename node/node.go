package node

import (
	"net"
	"time"

	"google.golang.org/grpc"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/consensus"
	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/future"
	"github.com/ultiledger/go-ultiledger/ledger"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/peer"
	"github.com/ultiledger/go-ultiledger/rpc"
	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
	"github.com/ultiledger/go-ultiledger/tx"
	"github.com/ultiledger/go-ultiledger/ultpb"
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
	tm     *tx.Manager
	engine *consensus.Engine

	// channel for stopping all the subroutines
	stopChan chan struct{}

	// futures for task with error responses
	txFuture     chan *future.Tx
	peerFuture   chan *future.Peer
	stmtFuture   chan *future.Statement
	quorumFuture chan *future.Quorum
	txsetFuture  chan *future.TxSet
	txsFuture    chan *future.TxStatus
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

	// peer and account managers are independent
	pm := peer.NewManager(conf.Peers, addr, nodeID)
	am := account.NewManager(store)

	// tx manager depends on peer and account manager
	txCtx := &tx.ManagerContext{
		Store:       store,
		PM:          pm,
		AM:          am,
		BaseReserve: ledger.GenesisBaseReserve,
		Seed:        seed,
	}
	tm := tx.NewManager(txCtx)

	// ledger manager depends on account and tx manager
	lm := ledger.NewManager(store, am, tm)

	stopChan := make(chan struct{})

	// construct consensus engine context and create consensus engine
	engineCtx := &consensus.EngineContext{
		Store:  store,
		Seed:   seed,
		NodeID: nodeID,
		PM:     pm,
		AM:     am,
		LM:     lm,
		TM:     tm,
		Quorum: conf.Quorum,
	}
	engine := consensus.NewEngine(engineCtx)

	txFuture := make(chan *future.Tx)
	peerFuture := make(chan *future.Peer)
	stmtFuture := make(chan *future.Statement)
	txsFuture := make(chan *future.TxStatus)
	quorumFuture := make(chan *future.Quorum)
	txsetFuture := make(chan *future.TxSet)

	// construct node server context and create node server
	serverCtx := &rpc.ServerContext{
		Addr:           addr,
		NodeID:         nodeID,
		Seed:           seed,
		PeerFuture:     peerFuture,
		TxFuture:       txFuture,
		StmtFuture:     stmtFuture,
		TxStatusFuture: txsFuture,
		QuorumFuture:   quorumFuture,
		TxSetFuture:    txsetFuture,
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
		tm:        tm,
		engine:    engine,
		addr:      addr,
		nodeID:    nodeID,
		startTime: time.Now().Unix(),
		stopChan:  stopChan,
	}

	return node
}

// Start checks the provided configurations, if the config is valid,
// it will trigger sub goroutines to do the sub tasks.
func (n *Node) Start() error {
	// start node server
	go n.serveNode()

	// start node server event loop
	go n.eventLoop()

	// start tx result event loop
	go n.txResultLoop()

	// start peer manager
	n.pm.Start()

	// start consensus engine
	n.engine.Start()

	select {}
	return nil
}

// Restart checks the provided configurations, if the config is valid,
// it will trigger sub goroutines to do the sub tasks.
func (n *Node) Restart() error {
	return nil
}

// Close node by signaling all the goroutines to stop
func (n *Node) Stop() {
	close(n.stopChan)
	n.pm.Stop()
	n.engine.Stop()
}

func (n *Node) txResultLoop() {
	for {
		select {
		case tr := <-n.lm.Ready():
			// get tx key
			hash, _ := ultpb.SHA256HashBytes(tr.Tx)
			k := &crypto.ULTKey{
				Code: crypto.KeyTypeTransaction,
				Hash: hash,
			}
			txKey := crypto.EncodeKey(k)

			// update tx status
			status := &rpcpb.TxStatus{}
			if tr.Err != nil {
				status.StatusCode = rpcpb.TxStatusCode_FAILED
				status.ErrorMessage = tr.Err.Error()
			} else {
				status.StatusCode = rpcpb.TxStatusCode_CONFIRMED
			}

			err := n.tm.UpdateTxStatus(txKey, status)
			if err != nil {
				log.Errorw("update tx status failed: %v", err, "tx", txKey)
			}
		case <-n.stopChan:
			log.Info("shutdown event loop")
			return
		}
	}
}

// Event loop for processing server received messages
func (n *Node) eventLoop() {
	// listening for node server events
	for {
		select {
		case txf := <-n.txFuture:
			err := n.tm.AddTx(txf.TxKey, txf.Tx)
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
		case qf := <-n.quorumFuture:
			quorum, err := n.engine.GetQuorum(qf.QuorumHash)
			if err != nil {
				log.Errorf("query quorum failed: %v", err)
			}
			qf.Quorum = quorum
			qf.Respond(err)
		case txf := <-n.txsetFuture:
			txset, err := n.engine.GetTxSet(txf.TxSetHash)
			if err != nil {
				log.Errorf("query txset failed: %v", err)
			}
			txf.TxSet = txset
			txf.Respond(err)
		case txs := <-n.txsFuture:
			txstatus, err := n.tm.GetTxStatus(txs.TxKey)
			if err != nil {
				log.Errorw("query tx status failed: %v", err, "tx", txs.TxKey)
			}
			txs.TxStatus = txstatus
			txs.Respond(err)
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
