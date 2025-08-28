package peer

import (
	"ledger/account"
	"strconv"
	"sync"
	"testing"
	"time"
)

func compareLedgers(l1, l2 *account.Ledger) bool {
	if len(l1.Accounts) != len(l2.Accounts) {
		return false
	}
	for acc, bal := range l1.Accounts {
		if l2.Accounts[acc] != bal {
			return false
		}
	}
	return true
}

func verifyLedgerConsistency(t *testing.T, peers []*Peer) bool {
	refLedger := peers[0].GetLedger()

	for i := 1; i < len(peers); i++ {
		ledger := peers[i].GetLedger()
		if len(refLedger.Accounts) != len(ledger.Accounts) {
			t.Errorf("Ledger size mismatch between peer %d and peer 0", i)
			return false
		}
		for acc, bal := range refLedger.Accounts {
			if ledger.Accounts[acc] != bal {
				t.Errorf("Account %s balance mismatch between peer %d and peer 0", acc, i)
				return false
			}
		}
	}
	return true
}

func createTestNetwork(t *testing.T, numPeers int, basePort int) []*Peer {
	peers := make([]*Peer, numPeers)

	for i := range numPeers {
		peers[i] = NewPeer("localhost", basePort+i)
		err := peers[i].Start()
		if err != nil {
			t.Fatalf("Failed to start peer %d: %v", i, err)
		}
		if i > 0 {
			err = peers[i].Connect("localhost", basePort)
			if err != nil {
				t.Fatalf("Peer %d failed to connect to peer 0: %v", i, err)
			}
		}
	}

	return peers
}

func cleanupPeers(peers []*Peer) {
	for _, peer := range peers {
		peer.Disconnect()
	}
}

func floodTransactionAndCompute(peer *Peer, tx *account.Transaction, computedLedge *account.Ledger) {
	peer.FloodTransaction(tx)
	computedLedge.Transaction(tx)
}

func TestCreateNetwork(t *testing.T) {
	numPeers := 5
	peers := createTestNetwork(t, numPeers, 10000)
	defer cleanupPeers(peers)
}

func TestCreateBigNetwork(t *testing.T) {
	numPeers := 100
	peers := createTestNetwork(t, numPeers, 10000)
	defer cleanupPeers(peers)
}

func TestReuseNetwork(t *testing.T) {
	numPeers := 5
	peers := createTestNetwork(t, numPeers, 10000)
	cleanupPeers(peers)
	peers = createTestNetwork(t, numPeers, 10000)
	defer cleanupPeers(peers)
}

func Go(wg *sync.WaitGroup, f func()) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		f()
	}()
}

func TestLedgerConsistency(t *testing.T) {
	peers := createTestNetwork(t, 5, 10000)
	defer cleanupPeers(peers)

	tx1 := &account.Transaction{
		ID:     "tx1",
		From:   "Alice",
		To:     "Bob",
		Amount: 50,
	}
	tx2 := &account.Transaction{
		ID:     "tx1",
		From:   "Bob",
		To:     "Alice",
		Amount: 50,
	}
	tx3 := &account.Transaction{
		ID:     "tx1",
		From:   "Alice",
		To:     "Carol",
		Amount: 100,
	}

	/*
		wg := sync.WaitGroup{}
		Go(&wg, func() { peers[0].FloodTransaction(tx1) })
		Go(&wg, func() { peers[0].FloodTransaction(tx1) })
		Go(&wg, func() { peers[1].FloodTransaction(tx1) })
		Go(&wg, func() { peers[2].FloodTransaction(tx3) })
		Go(&wg, func() { peers[1].FloodTransaction(tx2) })
		wg.Wait()
	*/
	go peers[0].FloodTransaction(tx1)
	go peers[0].FloodTransaction(tx1)
	go peers[1].FloodTransaction(tx1)
	go peers[2].FloodTransaction(tx3)
	go peers[1].FloodTransaction(tx2)

	time.Sleep(100 * time.Millisecond) // Wait for transactions to propagate

	if !verifyLedgerConsistency(t, peers) {
		t.Errorf("Ledgers are inconsistent after transactions")
	}
}

func TestRandomLedgerConsistency(t *testing.T) {
	numPeers := 5
	numTransactions := 10
	names := []string{"Alice", "Bob", "Carol", "Dave", "Eve"}

	peers := createTestNetwork(t, numPeers, 10000)
	defer cleanupPeers(peers)

	computedLedger := account.MakeLedger()

	for i := 0; i < numTransactions; i++ {
		randFrom := names[i%len(names)]
		randTo := names[(i+1)%len(names)]
		randAmount := (i + 1) * 10
		tx := &account.Transaction{
			ID:     "tx" + strconv.Itoa(i),
			From:   randFrom,
			To:     randTo,
			Amount: randAmount,
		}
		peerIndex := i % len(peers)
		go floodTransactionAndCompute(peers[peerIndex], tx, computedLedger)
		//go peers[peerIndex].FloodTransaction(tx)
		// computedLedger.Transaction(tx)
	}

	time.Sleep(1 * time.Second) // Wait for transactions to propagate

	if !verifyLedgerConsistency(t, peers) {
		t.Errorf("Ledgers are inconsistent after random transactions")
	}

	if !compareLedgers(peers[0].GetLedger(), computedLedger) {
		t.Errorf("Peers' ledgers do not match computed ledger")
	}
}
func _TestLateJoiningPeers(t *testing.T) {
	numPeersGroup1 := 5 // Connected before the transactions are fired
	numPeersGroup2 := 5 // Connected just after the transactions are fired
	peers_group1 := createTestNetwork(t, numPeersGroup1, 10000)
	defer cleanupPeers(peers_group1)
	base_tx := &account.Transaction{
		ID:     "tx",
		From:   "Alice",
		To:     "Bob",
		Amount: 100,
	}
	for i, peer := range peers_group1 {
		tx := *base_tx
		tx.ID = tx.ID + strconv.Itoa(i)
		peer.FloodTransaction(&tx)
	}
	peers_group2 := createTestNetwork(t, numPeersGroup2, 20000)
	defer cleanupPeers(peers_group2)
	time.Sleep(1 * time.Second)

	peers_merged := append(peers_group1, peers_group2...)

	if !verifyLedgerConsistency(t, peers_group1) {
		t.Errorf("Ledgers are inconsistent in network group 1(even excluding late joining peers)")
	}

	if !verifyLedgerConsistency(t, peers_merged) {
		t.Errorf("Ledgers are inconsistent in merged network(containing late joining peers)")
	}
}
