package p2p

import (
	"fmt"
	"math/rand"
	"net"
	"os"

	crypto "github.com/tendermint/go-crypto"
	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"

	cfg "github.com/bytom/config"
)

//------------------------------------------------------------------
// Switches connected via arbitrary net.Conn; useful for testing

// Returns n switches, connected according to the connect func.
// If connect==Connect2Switches, the switches will be fully connected.
// initSwitch defines how the ith switch should be initialized (ie. with what reactors).
// NOTE: panics if any switch fails to start.
func MakeConnectedSwitches(cfg *cfg.P2PConfig, n int, initSwitch func(*Switch) *Switch, connect func([]*Switch, int, int)) []*Switch {
	switches := make([]*Switch, n)
	for i := 0; i < n; i++ {
		switches[i] = makeSwitch(cfg, i, "testing", "123.123.123", initSwitch)
	}

	if err := StartSwitches(switches); err != nil {
		panic(err)
	}

	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			connect(switches, i, j)
		}
	}

	return switches
}

var PanicOnAddPeerErr = false

// Will connect switches i and j via net.Pipe()
// Blocks until a conection is established.
// NOTE: caller ensures i and j are within bounds
func Connect2Switches(switches []*Switch, i, j int) {
	switchI := switches[i]
	switchJ := switches[j]
	c1, c2 := net.Pipe()
	doneCh := make(chan struct{})
	go func() {
		err := switchI.addPeerWithConnection(c1)
		if PanicOnAddPeerErr && err != nil {
			panic(err)
		}
		doneCh <- struct{}{}
	}()
	go func() {
		err := switchJ.addPeerWithConnection(c2)
		if PanicOnAddPeerErr && err != nil {
			panic(err)
		}
		doneCh <- struct{}{}
	}()
	<-doneCh
	<-doneCh
}

func StartSwitches(switches []*Switch) error {
	for _, s := range switches {
		_, err := s.Start() // start switch and reactors
		if err != nil {
			return err
		}
	}
	return nil
}

func StopAndClean(switches []*Switch) {
	for i, s := range switches {
		s.Stop()
		dir := fmt.Sprintf("trustDB%d", i)
		os.RemoveAll(dir)
	}
}

func makeSwitch(cfg *cfg.P2PConfig, i int, network, version string, initSwitch func(*Switch) *Switch) *Switch {
	privKey := crypto.GenPrivKeyEd25519()
	// new switch, add reactors
	name := fmt.Sprintf("trustDB%d", i)
	trustHistoryDB := dbm.NewDB(name, "leveldb", name)
	s := NewSwitch(cfg, trustHistoryDB)
	if initSwitch != nil {
		s = initSwitch(s)
	}
	s.SetNodeInfo(&NodeInfo{
		PubKey:     privKey.PubKey().Unwrap().(crypto.PubKeyEd25519),
		Moniker:    cmn.Fmt("switch%d", i),
		Network:    network,
		Version:    version,
		RemoteAddr: cmn.Fmt("%v:%v", network, rand.Intn(64512)+1023),
		ListenAddr: cmn.Fmt("%v:%v", network, rand.Intn(64512)+1023),
	})
	s.SetNodePrivKey(privKey)
	return s
}

// DialPeer connect to address, the only difference with DialPeerWithAddress
// is that we can have multi connection to same IP
func (sw *Switch) DialPeer(addr *NetAddress, persistent bool) (*Peer, error) {
	if err := sw.checkBannedPeer(addr.IP.String()); err != nil {
		return nil, err
	}
	sw.dialing.Set(addr.IP.String(), addr)
	defer sw.dialing.Delete(addr.IP.String())

	peer, err := newOutboundPeerWithConfig(addr, sw.reactorsByCh, sw.chDescs, sw.StopPeerForError, sw.nodePrivKey, sw.peerConfig)
	if err != nil {
		return nil, err
	}
	if persistent {
		peer.makePersistent()
	}
	peer.SetLogger(sw.Logger.With("peer", addr))
	err = sw.AddPeer(peer)
	if err != nil {
		peer.CloseConn()
		return nil, err
	}
	return peer, nil
}
