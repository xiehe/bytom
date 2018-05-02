package netsync

import (
	"testing"
	"time"

	"github.com/bytom/protocol/bc/types"
)

func TestUnmatchVersion(t *testing.T) {
	managers, err := createSyncManagers(2)
	defer Clean()
	if err != nil {
		t.Fatal(err)
	}

	startAll(managers)
	// wait for start
	time.Sleep(2 * time.Second)

	peerNodeInfo := managers[1].NodeInfo()
	peerNodeInfo.Version = "0.5.7"
	if err := managers[0].connect(managers[1], false); err == nil {
		t.Fatal("connect unmatch version node success")
	}
	stopAll(managers)
}

func TestConnectPeer(t *testing.T) {
	managers, err := createSyncManagers(2)
	defer Clean()
	if err != nil {
		t.Fatal(err)
	}

	startAll(managers)
	// wait for start
	time.Sleep(2 * time.Second)
	if err := managers[0].connect(managers[1], false); err != nil {
		t.Fatal(err)
	}

	if len(managers[0].peers) != 1 {
		t.Fatalf("node0 connect peer failed")
	}

	if len(managers[1].peers) != 1 {
		t.Fatal("node1 connect peer failed")
	}

	if err := managers[0].disconnect(managers[1]); err != nil {
		t.Fatal(err)
	}

	if len(managers[0].peers) != 0 {
		t.Fatalf("node0 disconnect peer failed")
	}

	if len(managers[1].peers) != 0 {
		t.Fatalf("node1 disconnect peer failed")
	}

	stopAll(managers)
}

func TestSyncBlock(t *testing.T) {
	managers, err := createSyncManagers(3)
	defer Clean()
	if err != nil {
		t.Fatal(err)
	}

	startAll(managers)
	// wait for start
	time.Sleep(2 * time.Second)

	if err := managers[0].connect(managers[1], false); err != nil {
		t.Fatal(err)
	}

	if err := managers[1].connect(managers[2], false); err != nil {
		t.Fatal(err)
	}

	blocks, err := managers[0].generateBlocks(5)
	if err != nil {
		t.Fatal(err)
	}

	// now, the height of managers[0].chain is 5, start sync
	for _, block := range blocks {
		hash := block.Hash()
		managers[0].broadcastBlock(&hash)
	}

	// wait for sync completed
	predict := func() bool {
		bestHeight := managers[0].chain.BestBlockHeight()
		bestHash := managers[0].chain.BestBlockHash()

		// check peer chain status
		nodesIndex := []int{1, 2}
		for _, idx := range nodesIndex {
			if managers[idx].chain.BestBlockHeight() != bestHeight ||
				*managers[idx].chain.BestBlockHash() != *bestHash {
				return false
			}
		}
		return true
	}

	if err := waitFor(5, 1, predict); err != nil {
		t.Fatalf("TestSyncBlock failed: %s", err)
	}
	stopAll(managers)
}

func TestSendInvalidBlock(t *testing.T) {
	managers, err := createSyncManagers(2)
	defer Clean()
	if err != nil {
		t.Fatal(err)
	}

	startAll(managers)
	// wait for start
	time.Sleep(2 * time.Second)

	if err := managers[0].connect(managers[1], false); err != nil {
		t.Fatal(err)
	}

	genesisBlock, _ := managers[0].chain.GetBlockByHeight(0)
	for i := 0; i < 6; i++ {
		newBlock := &types.Block{
			BlockHeader: types.BlockHeader{
				Height:            1,
				PreviousBlockHash: genesisBlock.Hash(),
				Nonce:             uint64(i),
			},
		}

		_, err = managers[0].Peers().BroadcastMinedBlock(newBlock)
		if err != nil {
			t.Fatal(err)
		}
	}

	// wait for sync
	time.Sleep(2 * time.Second)
	if managers[1].Switch().Peers().Size() != 0 {
		t.Fatal("banned peer still alive")
	}
}
