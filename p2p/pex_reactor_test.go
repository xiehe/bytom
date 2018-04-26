package p2p

import (
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	wire "github.com/tendermint/go-wire"
	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"
	"github.com/tendermint/tmlibs/log"

	cfg "github.com/bytom/config"
)

func TestPEXReactorBasic(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(err)
	defer os.RemoveAll(dir)
	book := NewAddrBook(dir+"addrbook.json", true)
	book.SetLogger(log.TestingLogger())

	r := NewPEXReactor(book, nil)
	r.SetLogger(log.TestingLogger())

	assert.NotNil(r)
	assert.NotEmpty(r.GetChannels())
}

func TestPEXReactorAddRemovePeer(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(err)
	defer os.RemoveAll(dir)
	book := NewAddrBook(dir+"addrbook.json", true)
	book.SetLogger(log.TestingLogger())

	peer := createRandomPeer(false)
	trustDB := dbm.NewDB("trustDB", "leveldb", "trustDB")
	defer os.RemoveAll("trustDB")
	sw := NewSwitch(cfg.DefaultP2PConfig(), trustDB)
	sw.peers.Add(peer)

	r := NewPEXReactor(book, sw)
	r.SetLogger(log.TestingLogger())

	size := book.Size()

	r.AddPeer(peer)
	assert.Equal(size+1, book.Size())

	r.RemovePeer(peer, "peer not available")
	assert.Equal(size+1, book.Size())

	outboundPeer := createRandomPeer(true)

	r.AddPeer(outboundPeer)
	assert.Equal(size+1, book.Size(), "outbound peers should not be added to the address book")

	r.RemovePeer(outboundPeer, "peer not available")
	assert.Equal(size+1, book.Size())
}

/*
NOTE: can't connect to self(same ip, 127.0.0.1), it would be better if it's configurable
func TestPEXReactorRunning(t *testing.T) {
	require := require.New(t)

	N := 2
	switches := make([]*Switch, N)

	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(err)
	defer os.RemoveAll(dir)
	book := NewAddrBook(dir+"addrbook.json", false)
	book.SetLogger(log.TestingLogger())

	// create switches
	for i := 0; i < N; i++ {
		switches[i] = makeSwitch(config, i, "127.0.0.1", "123.123.123", func(sw *Switch) *Switch {
			sw.SetLogger(log.TestingLogger().With("switch", i))

			r := NewPEXReactor(book, sw)
			r.SetLogger(log.TestingLogger())
			r.SetEnsurePeersPeriod(250 * time.Millisecond)
			sw.AddReactor("pex", r)
			return sw
		})
	}

	// fill the address book and add listeners
	for _, s := range switches {
		addr, _ := NewNetAddressString(s.NodeInfo().ListenAddr)
		book.AddAddress(addr, addr)
		s.AddListener(NewDefaultListener("tcp", s.NodeInfo().ListenAddr, true, log.TestingLogger()))
	}

	for _, address := range book.addrLookup {
		address.LastAttempt = address.LastAttempt.Add(-2 * time.Minute)
	}

	// start switches
	for _, s := range switches {
		_, err := s.Start() // start switch and reactors
		require.Nil(err)
	}

	// sigh, we have to wait >10s at least, refer to pex_reactor.OnStart() start, which started with delay
	time.Sleep(12 * time.Second)

	// check peers are connected after some time
	for _, s := range switches {
		outbound, inbound, _ := s.NumPeers()
		if outbound+inbound == 0 {
			t.Errorf("%v expected to be connected to at least one peer", s.NodeInfo().ListenAddr)
		}
	}
	StopAndClean(switches)
}
*/

func TestPEXReactorReceive(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(err)
	defer os.RemoveAll(dir)
	book := NewAddrBook(dir+"addrbook.json", true)
	book.SetLogger(log.TestingLogger())

	peer := createRandomPeer(false)
	trustDB := dbm.NewDB("trustDB", "leveldb", "trustDB")
	defer os.RemoveAll("trustDB")
	sw := NewSwitch(cfg.DefaultP2PConfig(), trustDB)
	sw.peers.Add(peer)

	r := NewPEXReactor(book, sw)
	r.SetLogger(log.TestingLogger())

	size := book.Size()
	netAddr, _ := NewNetAddressString(peer.ListenAddr)
	addrs := []*NetAddress{netAddr}
	msg := wire.BinaryBytes(struct{ PexMessage }{&pexAddrsMessage{Addrs: addrs}})
	r.Receive(PexChannel, peer, msg)
	assert.Equal(size+1, book.Size())

	msg = wire.BinaryBytes(struct{ PexMessage }{&pexRequestMessage{}})
	r.Receive(PexChannel, peer, msg)
}

func TestPEXReactorAbuseFromPeer(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	dir, err := ioutil.TempDir("", "pex_reactor")
	require.Nil(err)
	defer os.RemoveAll(dir)
	book := NewAddrBook(dir+"addrbook.json", true)
	book.SetLogger(log.TestingLogger())

	peer := createRandomPeer(false)
	trustDB := dbm.NewDB("trustDB", "leveldb", "trustDB")
	defer os.RemoveAll("trustDB")
	sw := NewSwitch(cfg.DefaultP2PConfig(), trustDB)
	sw.peers.Add(peer)

	r := NewPEXReactor(book, sw)
	r.SetLogger(log.TestingLogger())
	r.SetMaxMsgCountByPeer(5)

	msg := wire.BinaryBytes(struct{ PexMessage }{&pexRequestMessage{}})
	for i := 0; i < 10; i++ {
		r.Receive(PexChannel, peer, msg)
	}

	assert.True(r.ReachedMaxMsgCountForPeer(peer.ListenAddr))
}

func createRandomPeer(outbound bool) *Peer {
	addr := cmn.Fmt("%v.%v.%v.%v:46656", rand.Int()%256, rand.Int()%256, rand.Int()%256, rand.Int()%256)
	netAddr, _ := NewNetAddressString(addr)
	connection, _ := net.Pipe()
	mconn := NewMConnectionWithConfig(connection, nil, nil, nil, nil)
	mconn.RemoteAddress = netAddr
	p := &Peer{
		Key: cmn.RandStr(12),
		NodeInfo: &NodeInfo{
			ListenAddr: addr,
		},
		outbound: outbound,
		mconn:    mconn,
	}
	p.SetLogger(log.TestingLogger().With("peer", addr))
	p.BaseService = *cmn.NewBaseService(nil, "peer", p)
	return p
}
