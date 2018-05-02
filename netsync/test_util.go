package netsync

import (
	"fmt"
	"os"
	"time"

	cfg "github.com/bytom/config"
	"github.com/bytom/database/leveldb"
	"github.com/bytom/mining"
	"github.com/bytom/mining/tensority"
	"github.com/bytom/p2p"
	"github.com/bytom/protocol"
	"github.com/bytom/protocol/bc"
	"github.com/bytom/protocol/bc/types"

	dbm "github.com/tendermint/tmlibs/db"
)

const (
	p2pPortStart   = 46657
	testDir        = "/tmp/bytom_tests/"
	managerRootDir = "manager"
	ip             = "127.0.0.1"
)

// Clean used to clean all test data
func Clean() {
	os.RemoveAll(testDir)
}

func p2pPort(idx int) int {
	return p2pPortStart + idx
}

func rootDir(idx int) string {
	return fmt.Sprintf("%s%s%d", testDir, managerRootDir, idx)
}

func defaultConfig(idx int) *cfg.Config {
	return &cfg.Config{
		BaseConfig: cfg.BaseConfig{
			Moniker:           "anonymous",
			ProfListenAddress: "",
			RootDir:           rootDir(idx),
			ChainID:           "testnet",
			Mining:            false,
			DBBackend:         "leveldb",
			DBPath:            "data",
			KeysPath:          "keystore",
			HsmUrl:            "",
		},
		P2P: &cfg.P2PConfig{
			RootDir:          rootDir(idx),
			ListenAddress:    fmt.Sprintf("tcp://%s:%d", ip, p2pPort(idx)),
			AddrBook:         "addrbook.json",
			AddrBookStrict:   true,
			SkipUPNP:         true,
			MaxNumPeers:      50,
			HandshakeTimeout: 30,
			DialTimeout:      3,
			PexReactor:       false,
			Seeds:            "",
		},
		Wallet: &cfg.WalletConfig{
			Disable: true,
		},
		Auth: &cfg.RPCAuthConfig{
			Disable: true,
		},
		Web: &cfg.WebConfig{
			Closed: true,
		},
	}
}

func createSyncManagers(num int) ([]*mockedSyncManager, error) {
	managers := make([]*mockedSyncManager, 0)
	for i := 0; i < num; i++ {
		m, err := createSyncManager(i)
		if err != nil {
			return nil, err
		}
		managers = append(managers, m)
	}
	return managers, nil
}

func createSyncManager(index int) (*mockedSyncManager, error) {
	config := defaultConfig(index)
	coreDB := dbm.NewDB("chain_db", config.DBBackend, config.DBDir())
	store := leveldb.NewStore(coreDB)
	txPool := protocol.NewTxPool()
	chain, err := protocol.NewChain(store, txPool)
	if err != nil {
		return nil, err
	}

	newBlockCh := make(chan *bc.Hash)
	manager, err := NewSyncManager(config, chain, txPool, newBlockCh)
	if err != nil {
		return nil, err
	}

	return &mockedSyncManager{
		SyncManager: manager,
		index:       index,
		peers:       make(map[int]*p2p.Peer),
	}, nil
}

func startAll(managers []*mockedSyncManager) {
	for _, m := range managers {
		m.Start()
	}
}

func stopAll(managers []*mockedSyncManager) {
	for _, m := range managers {
		m.Stop()
	}
}

// blocked wait for predict return true
func waitFor(maxTimeout, interval int, predict func() bool) error {
	timeout := 0
	for timeout < maxTimeout {
		if predict() {
			return nil
		}
		time.Sleep(time.Duration(interval) * time.Second)
		timeout += interval
	}
	return fmt.Errorf("timeout")
}

type mockedSyncManager struct {
	*SyncManager
	index int
	peers map[int]*p2p.Peer
}

func (m *mockedSyncManager) listenAddr() (*p2p.NetAddress, error) {
	port := p2pPort(m.index)
	addr := fmt.Sprintf("%s:%d", ip, port)
	return p2p.NewNetAddressString(addr)
}

// Connect connect to remote, update peers of both manager
func (m *mockedSyncManager) connect(manager *mockedSyncManager, persistent bool) error {
	addr, err := manager.listenAddr()
	if err != nil {
		return err
	}

	// used for update peers of manager, it is inbound for manager
	peerSw := manager.Switch()
	managerPeers := peerSw.Peers().List()
	peerNum := len(managerPeers)

	sw := m.Switch()
	p, err := sw.DialPeer(addr, persistent)
	if err != nil {
		return err
	}

	m.peers[manager.index] = p
	// wait for connecting...
	predict := func() bool {
		managerPeers = peerSw.Peers().List()
		return len(managerPeers) == peerNum+1
	}

	if err := waitFor(5, 1, predict); err != nil {
		return err
	}
	managerPeers = peerSw.Peers().List()
	manager.peers[m.index] = managerPeers[len(managerPeers)-1]
	return nil
}

// Disconnect disconnect from remote
func (m *mockedSyncManager) disconnect(manager *mockedSyncManager) error {
	index := manager.index
	if _, ok := m.peers[index]; !ok {
		return fmt.Errorf("can't find peer")
	}

	peerSw := manager.Switch()
	managerPeers := peerSw.Peers().List()
	peerNum := len(managerPeers)

	sw := m.Switch()
	sw.StopPeerGracefully(m.peers[index])
	delete(m.peers, index)

	// wait for disconnecting...
	predict := func() bool {
		managerPeers = peerSw.Peers().List()
		return len(managerPeers) == peerNum-1
	}

	if err := waitFor(5, 1, predict); err != nil {
		return err
	}
	managerPeers = peerSw.Peers().List()
	delete(manager.peers, m.index)
	return nil
}

func (m *mockedSyncManager) broadcastBlock(hash *bc.Hash) {
	m.newBlockCh <- hash
}

func (m *mockedSyncManager) broadcastTx(tx *types.Tx) {
	newTxCh := m.chain.GetTxPool().GetNewTxCh()
	newTxCh <- tx
}

func (m *mockedSyncManager) solve(block *types.Block) error {
	seed, err := m.chain.CalcNextSeed(&block.PreviousBlockHash)
	if err != nil {
		return err
	}

	hash := block.Hash()
	tensority.AIHash.AddCache(&hash, seed, &bc.Hash{})
	return nil
}

func (m *mockedSyncManager) generateBlocks(num int) ([]*types.Block, error) {
	txPool := m.chain.GetTxPool()
	blocks := make([]*types.Block, 0, num)
	for i := 0; i < num; i++ {
		block, err := mining.NewBlockTemplate(m.chain, txPool, nil)
		if err != nil {
			return nil, err
		}

		// change timestamp manually
		block.Timestamp = m.chain.BestBlockHeader().Timestamp + 1
		if err := m.solve(block); err != nil {
			return nil, err
		}

		_, err = m.chain.ProcessBlock(block)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}
