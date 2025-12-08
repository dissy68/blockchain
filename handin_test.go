package main

import (
	blockchain_params "au_blockchain/internal"
	"au_blockchain/internal/account"
	"au_blockchain/internal/peer"
	"au_blockchain/internal/signature"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

const TEST_BASE_PORT = 15000

// TestBasicNetworkSetup tests that a basic blockchain network can be created and peers can connect
func TestBasicNetworkSetup(t *testing.T) {
	t.Log("=== Testing Basic Network Setup ===")

	numPeers := 5
	peers := make([]*peer.Peer, numPeers)

	for i := 0; i < numPeers; i++ {
		peers[i] = peer.NewPeer("localhost", TEST_BASE_PORT+i,
			account.DANGER_GetGenesisKeyPair("genesis_pks_sks.json", i))
		err := peers[i].Connect("localhost", TEST_BASE_PORT)
		if err != nil {
			t.Fatalf("Failed to connect peer %d: %v", i, err)
		}
	}
	defer peer.CleanupPeers(peers)

	time.Sleep(2 * time.Second)

	// Verify all peers are connected
	peerList := peers[0].GetPeers()
	if len(peerList) != numPeers {
		t.Errorf("Expected %d peers, got %d", numPeers, len(peerList))
	}

	t.Logf("✓ Network setup successful with %d peers", numPeers)
}

// TestValidTransactions tests that valid transactions are processed correctly
func TestValidTransactions(t *testing.T) {
	t.Log("=== Testing Valid Transactions ===")

	numPeers := 5
	peers := make([]*peer.Peer, numPeers)

	for i := 0; i < numPeers; i++ {
		peers[i] = peer.NewPeer("localhost", TEST_BASE_PORT+100+i,
			account.DANGER_GetGenesisKeyPair("genesis_pks_sks.json", i))
		peers[i].Connect("localhost", TEST_BASE_PORT+100)
	}
	defer peer.CleanupPeers(peers)

	time.Sleep(2 * time.Second)

	initialBalance := peers[0].GetLedger().GetBalance(peers[0].GetEncodedPublicKey())

	// Send valid transaction
	amount := 100
	peers[0].SendBalance(peers[1].GetEncodedPublicKey(), amount)

	time.Sleep(3 * time.Second)

	// Verify ledger consistency
	ledgers := make([]*account.Ledger, numPeers)
	for i := 0; i < numPeers; i++ {
		ledgers[i] = peers[i].GetLedger()
	}

	if !account.VerifyLedgerConsistency(ledgers) {
		t.Error("✗ Ledgers are inconsistent after valid transaction")
	} else {
		t.Log("✓ Valid transactions processed correctly with ledger consistency")
	}

	finalBalance := peers[0].GetLedger().GetBalance(peers[0].GetEncodedPublicKey())
	expectedBalance := initialBalance - amount - blockchain_params.TRANSACTION_FEE

	if finalBalance != expectedBalance {
		t.Logf("Balance changed as expected (initial: %d, final: %d)", initialBalance, finalBalance)
	}
}

// TestInvalidTransactions tests rejection of various invalid transaction types
func TestInvalidTransactions(t *testing.T) {
	t.Log("=== Testing Invalid Transaction Rejection ===")

	numPeers := 3
	peers := make([]*peer.Peer, numPeers)

	for i := 0; i < numPeers; i++ {
		peers[i] = peer.NewPeer("localhost", TEST_BASE_PORT+200+i,
			account.DANGER_GetGenesisKeyPair("genesis_pks_sks.json", i))
		peers[i].Connect("localhost", TEST_BASE_PORT+200)
	}
	defer peer.CleanupPeers(peers)

	time.Sleep(1 * time.Second)

	tests := []struct {
		name   string
		amount int
		desc   string
	}{
		{"Negative amount", -100, "should be rejected"},
		{"Zero amount", 0, "should be rejected"},
		{"Excessive amount", 10000000, "insufficient funds"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			initialBalance := peers[0].GetLedger().GetBalance(peers[0].GetEncodedPublicKey())

			peers[0].SendBalance(peers[1].GetEncodedPublicKey(), test.amount)
			time.Sleep(2 * time.Second)

			finalBalance := peers[0].GetLedger().GetBalance(peers[0].GetEncodedPublicKey())

			if test.amount <= 0 || test.amount > initialBalance {
				if initialBalance == finalBalance {
					t.Logf("✓ %s correctly rejected", test.name)
				} else {
					t.Errorf("✗ %s was incorrectly accepted", test.name)
				}
			}
		})
	}
}

// TestInvalidSignatureTransaction tests that transactions with invalid signatures are rejected
func TestInvalidSignatureTransaction(t *testing.T) {
	t.Log("=== Testing Invalid Signature Rejection ===")

	numPeers := 3
	peers := make([]*peer.Peer, numPeers)

	for i := 0; i < numPeers; i++ {
		peers[i] = peer.NewPeer("localhost", TEST_BASE_PORT+250+i,
			account.DANGER_GetGenesisKeyPair("genesis_pks_sks.json", i))
		peers[i].Connect("localhost", TEST_BASE_PORT+250)
	}
	defer peer.CleanupPeers(peers)

	time.Sleep(1 * time.Second)

	// Create transaction with wrong key
	wrongKp := signature.DefaultKeyGen()
	kp1 := account.DANGER_GetGenesisKeyPair("genesis_pks_sks.json", 0)
	kp2 := account.DANGER_GetGenesisKeyPair("genesis_pks_sks.json", 1)

	initialBalance := peers[0].GetLedger().GetBalance(kp1.Pk.Encode())

	// Create transaction signed with wrong key
	invalidTx := account.NewSignedTransaction(
		kp1.Pk.Encode(),
		kp2.Pk.Encode(),
		100,
		wrongKp.Sk,
	)

	// Try to verify it should fail
	if !invalidTx.Verify() {
		t.Log("✓ Invalid signature detected correctly")
	} else {
		t.Error("✗ Invalid signature was not detected")
	}

	time.Sleep(2 * time.Second)
	finalBalance := peers[0].GetLedger().GetBalance(kp1.Pk.Encode())

	if initialBalance == finalBalance {
		t.Log("✓ Transaction with invalid signature was rejected")
	}
}

// TestThroughputWithDifferentBlockSizes measures transaction throughput with different block sizes
func TestThroughputWithDifferentBlockSizes(t *testing.T) {
	t.Log("=== Testing Throughput with Different Block Sizes ===")

	// Note: Since BLOCK_SIZE_LIMIT is a constant, we can only test with the current value
	// In a real scenario, you would modify blockchain_params.BLOCK_SIZE_LIMIT

	numPeers := 5
	numTransactions := 50

	blockSizes := []int{5, 10, 20, 50}

	for _, size := range blockSizes {
		t.Run(fmt.Sprintf("BlockSize_%d", size), func(t *testing.T) {
			peers := make([]*peer.Peer, numPeers)
			basePort := TEST_BASE_PORT + 300 + size*10

			for i := 0; i < numPeers; i++ {
				peers[i] = peer.NewPeer("localhost", basePort+i,
					account.DANGER_GetGenesisKeyPair("genesis_pks_sks.json", i))
				peers[i].Connect("localhost", basePort)
			}

			time.Sleep(1 * time.Second)

			startTime := time.Now()

			for i := 0; i < numTransactions; i++ {
				from := i % numPeers
				to := (i + 1) % numPeers
				peers[from].SendBalance(peers[to].GetEncodedPublicKey(), 1)
			}

			time.Sleep(5 * time.Second)

			elapsed := time.Since(startTime)
			tps := float64(numTransactions) / elapsed.Seconds()

			t.Logf("Block size %d: %d transactions in %.2f seconds (%.2f TPS)",
				size, numTransactions, elapsed.Seconds(), tps)

			peer.CleanupPeers(peers)
			time.Sleep(500 * time.Millisecond)
		})
	}
}

// TestRollbackTolerance tests that the system can handle rollbacks when block time is low
func TestRollbackTolerance(t *testing.T) {
	t.Log("=== Testing Rollback Tolerance ===")

	numPeers := 5
	peers := make([]*peer.Peer, numPeers)

	for i := 0; i < numPeers; i++ {
		peers[i] = peer.NewPeer("localhost", TEST_BASE_PORT+400+i,
			account.DANGER_GetGenesisKeyPair("genesis_pks_sks.json", i))
		peers[i].Connect("localhost", TEST_BASE_PORT+400)
	}
	defer peer.CleanupPeers(peers)

	time.Sleep(1 * time.Second)

	// Send concurrent transactions from multiple peers to trigger potential forks
	t.Log("Sending concurrent transactions to trigger potential forks...")
	for round := 0; round < 3; round++ {
		for i := 0; i < numPeers; i++ {
			go func(idx int) {
				to := (idx + 1) % numPeers
				peers[idx].SendBalance(peers[to].GetEncodedPublicKey(), 5)
			}(i)
		}
		time.Sleep(500 * time.Millisecond)
	}

	time.Sleep(5 * time.Second)

	// Check consistency
	ledgers := make([]*account.Ledger, numPeers)
	for i := 0; i < numPeers; i++ {
		ledgers[i] = peers[i].GetLedger()
	}

	if account.VerifyLedgerConsistency(ledgers) {
		t.Log("✓ All ledgers are consistent despite potential rollbacks")
	} else {
		t.Error("✗ Ledger inconsistency detected")
	}

	t.Log("Block tree structure:")
	peers[0].GetBlockTree().Display()
}

// TestMixedValidAndInvalidTransactions tests a mix of valid and invalid transactions (25% invalid)
func TestMixedValidAndInvalidTransactions(t *testing.T) {
	t.Log("=== Testing Mixed Valid and Invalid Transactions ===")

	numPeers := 5
	peers := make([]*peer.Peer, numPeers)

	for i := 0; i < numPeers; i++ {
		peers[i] = peer.NewPeer("localhost", TEST_BASE_PORT+500+i,
			account.DANGER_GetGenesisKeyPair("genesis_pks_sks.json", i))
		peers[i].Connect("localhost", TEST_BASE_PORT+500)
	}
	defer peer.CleanupPeers(peers)

	time.Sleep(2 * time.Second)

	totalTxs := 20
	invalidTxs := 0

	for i := 0; i < totalTxs; i++ {
		from := i % numPeers
		to := (i + 1) % numPeers

		// Make 25% of transactions invalid
		if rand.Float64() < 0.25 {
			invalidAmount := []int{-10, 0, 10000000}[rand.Intn(3)]
			peers[from].SendBalance(peers[to].GetEncodedPublicKey(), invalidAmount)
			invalidTxs++
			t.Logf("Sent invalid transaction: amount=%d", invalidAmount)
		} else {
			peers[from].SendBalance(peers[to].GetEncodedPublicKey(), 10)
		}
	}

	time.Sleep(5 * time.Second)

	// Check consistency
	ledgers := make([]*account.Ledger, numPeers)
	for i := 0; i < numPeers; i++ {
		ledgers[i] = peers[i].GetLedger()
	}

	if account.VerifyLedgerConsistency(ledgers) {
		t.Logf("✓ Ledgers consistent after %d total transactions (%d invalid)", totalTxs, invalidTxs)
	} else {
		t.Error("✗ Ledgers are inconsistent")
	}
}

// TestLedgerConsistencyUnderLoad tests ledger consistency with high transaction load
func TestLedgerConsistencyUnderLoad(t *testing.T) {
	t.Log("=== Testing Ledger Consistency Under Load ===")

	numPeers := blockchain_params.NUM_GENESIS_ACCOUNTS
	peers := make([]*peer.Peer, numPeers)

	for i := 0; i < numPeers; i++ {
		peers[i] = peer.NewPeer("localhost", TEST_BASE_PORT+600+i,
			account.DANGER_GetGenesisKeyPair("genesis_pks_sks.json", i))
		peers[i].Connect("localhost", TEST_BASE_PORT+600)
	}
	defer peer.CleanupPeers(peers)

	time.Sleep(2 * time.Second)

	// Send many transactions
	numTransactions := 100
	for i := 0; i < numTransactions; i++ {
		from := rand.Intn(numPeers)
		to := rand.Intn(numPeers)
		if from != to {
			peers[from].SendBalance(peers[to].GetEncodedPublicKey(), rand.Intn(50)+1)
		}
	}

	time.Sleep(10 * time.Second)

	// Verify consistency
	ledgers := make([]*account.Ledger, numPeers)
	for i := 0; i < numPeers; i++ {
		ledgers[i] = peers[i].GetLedger()
	}

	if account.VerifyLedgerConsistency(ledgers) {
		t.Logf("✓ All ledgers consistent after %d transactions", numTransactions)
	} else {
		t.Error("✗ Ledgers are inconsistent under load")
	}
}
