package node

import (
	"net"
	"strings"
	"time"

	b58 "github.com/mr-tron/base58/base58"
	"google.golang.org/grpc"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/consensus"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/db/boltdb"
	"github.com/ultiledger/go-ultiledger/exchange"
	"github.com/ultiledger/go-ultiledger/future"
	"github.com/ultiledger/go-ultiledger/ledger"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/peer"
	"github.com/ultiledger/go-ultiledger/rpc"
	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
	"github.com/ultiledger/go-ultiledger/tx"
)

// Node is the central controller for ultiledger
type Node struct {
	database db.Database

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
	txFuture      chan *future.Tx
	peerFuture    chan *future.Peer
	stmtFuture    chan *future.Statement
	ledgerFuture  chan *future.Ledger
	quorumFuture  chan *future.Quorum
	txsetFuture   chan *future.TxSet
	txsFuture     chan *future.TxStatus
	accountFuture chan *future.Account
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

	log.Infof("local IP is: %s", netAddr.String())

	addr := strings.Split(netAddr.String(), ":")[0] + ":" + conf.Port
	nodeID := conf.NodeID
	role := conf.Role
	seed := conf.Seed
	networkID := conf.NetworkID

	// create database store
	database := boltdb.New(conf.DBPath)

	// peer and account managers are independent
	pm := peer.NewManager(conf.Peers, addr, nodeID, conf.MaxPeers)
	am := account.NewManager(database, ledger.GenesisBaseReserve)
	em := exchange.NewManager(database, am)

	// tx manager depends on peer, account and exchange manager
	txCtx := &tx.ManagerContext{
		Database:    database,
		PM:          pm,
		AM:          am,
		EM:          em,
		BaseReserve: ledger.GenesisBaseReserve,
		Seed:        seed,
	}
	tm := tx.NewManager(txCtx)

	// ledger manager depends on account and tx manager
	lmCtx := &ledger.ManagerContext{
		Database: database,
		PM:       pm,
		AM:       am,
		TM:       tm,
	}
	lm := ledger.NewManager(lmCtx)

	stopChan := make(chan struct{})

	// construct consensus engine context and create consensus engine
	engineCtx := &consensus.EngineContext{
		Database:        database,
		Seed:            seed,
		NodeID:          nodeID,
		Role:            role,
		PM:              pm,
		AM:              am,
		LM:              lm,
		TM:              tm,
		Quorum:          conf.Quorum,
		ProposeInterval: conf.ProposeInterval,
	}
	engine := consensus.NewEngine(engineCtx)

	txFuture := make(chan *future.Tx)
	peerFuture := make(chan *future.Peer)
	stmtFuture := make(chan *future.Statement)
	txsFuture := make(chan *future.TxStatus)
	ledgerFuture := make(chan *future.Ledger)
	quorumFuture := make(chan *future.Quorum)
	txsetFuture := make(chan *future.TxSet)
	accountFuture := make(chan *future.Account)

	// construct node server context and create node server
	serverCtx := &rpc.ServerContext{
		NetworkID:      networkID,
		Addr:           addr,
		NodeID:         nodeID,
		Seed:           seed,
		PeerFuture:     peerFuture,
		TxFuture:       txFuture,
		StmtFuture:     stmtFuture,
		TxStatusFuture: txsFuture,
		LedgerFuture:   ledgerFuture,
		QuorumFuture:   quorumFuture,
		TxSetFuture:    txsetFuture,
		AccountFuture:  accountFuture,
	}
	nodeServer := rpc.NewNodeServer(serverCtx)

	// create local node
	node := &Node{
		config:    conf,
		database:  database,
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
func (n *Node) Start(newnode bool) {
	// start node server
	go n.serveNode()

	// start node server event loop
	go n.eventLoop()

	// start peer manager
	n.pm.Start()

	// start consensus engine
	n.engine.Start()

	// start ledger manager
	n.lm.Start()

	// start tx manager
	n.tm.Start()

	if newnode {
		err := n.lm.CreateGenesisLedger()
		if err != nil {
			log.Fatalf("create genesis ledger failed: %v", err)
		}
		netID, err := b58.Decode(n.config.NetworkID)
		if err != nil {
			log.Fatalf("decode network id failed: %v", err)
		}
		err = n.am.CreateMasterAccount(netID, ledger.GenesisTotalTokens, uint64(1))
		if err != nil {
			log.Fatalf("create master account failed: %v", err)
		}
		err = n.engine.Propose()
		if err != nil {
			log.Fatalf("propose new value failed: %v", err)
		}
	}

	for {
		select {
		case <-n.stopChan:
			return
		}
	}
}

// Close node by signaling all the goroutines to stop
func (n *Node) Stop() {
	close(n.stopChan)
	n.pm.Stop()
	n.engine.Stop()
	n.lm.Stop()
	n.tm.Stop()
}

// Event loop for processing messages from peers and internal queries.
func (n *Node) eventLoop() {
	// listening for node server events
	for {
		select {
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
		case lf := <-n.ledgerFuture:
			ledger, err := n.lm.GetLedger(lf.LedgerSeq)
			if err != nil {
				log.Errorf("query ledger failed: %v", err)
			}
			lf.Ledger = ledger
			lf.Respond(err)
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
		case <-n.stopChan:
			log.Info("shutdown event loop")
			return
		}
	}
}

// ServeNode starts a listener on the port and starts to accept external requests.
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
		case txf := <-n.txFuture:
			err := n.tm.AddTx(txf.TxKey, txf.Tx)
			if err != nil {
				log.Errorf("add tx failed: %v", err)
			}
			txf.Respond(err)
		case txs := <-n.txsFuture:
			txstatus, err := n.tm.GetTxStatus(txs.TxKey)
			if err != nil {
				log.Errorw("query tx status failed: %v", err, "tx", txs.TxKey)
			}
			txs.TxStatus = txstatus
			txs.Respond(err)
		case acc := <-n.accountFuture:
			account, err := n.am.GetAccount(n.database, acc.AccountID)
			if err != nil {
				log.Errorw("get account failed: %v", err, "accountID", acc.AccountID)
			}
			acc.Account = account
			acc.Respond(err)
		case <-n.stopChan:
			log.Infof("gracefully shutdown gRPC server")
			s.GracefulStop()
			return
		}
	}
}
