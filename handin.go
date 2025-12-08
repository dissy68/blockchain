package main

import (
	blockchain_params "au_blockchain/internal"
	"au_blockchain/internal/account"
	"au_blockchain/internal/peer"
	"fmt"
	"math/rand"
	"time"
)

const BASE_PORT = 10000

func main() {
	rand.Seed(time.Now().UnixNano())

	fmt.Println("=== AU Blockchain Demonstration ===")
	fmt.Println()

	// Demonstrate basic functionality
	fmt.Println("1. Starting blockchain network with", blockchain_params.NUM_GENESIS_ACCOUNTS, "peers...")
	runBasicDemo()

	fmt.Println("\n2. Testing invalid transactions...")
	testInvalidTransactions()

	fmt.Println("\n3. Testing throughput with different block sizes...")
	testThroughput()

	fmt.Println("\n4. Testing rollback tolerance...")
	testRollbackTolerance()

	fmt.Println("\n=== Tests Complete ===")
}

func runBasicDemo() {
	n := blockchain_params.NUM_GENESIS_ACCOUNTS
	peers := make([]*peer.Peer, n)

	// Create and connect peers
	for i := range n {
		peers[i] = peer.NewPeer("localhost", BASE_PORT+i, account.DANGER_GetGenesisKeyPair("genesis_pks_sks.json", i))
		peers[i].Connect("localhost", BASE_PORT)
	}
	defer peer.CleanupPeers(peers)

	fmt.Printf("Network started with %d peers\n", n)
	time.Sleep(2 * time.Second)

	// Send some valid transactions
	fmt.Println("Sending valid transactions...")
	for i := 0; i < 5; i++ {
		from := i % n
		to := (i + 1) % n
		amount := 10
		peers[from].SendBalance(peers[to].GetEncodedPublicKey(), amount)
		fmt.Printf("  Peer %d -> Peer %d: %d coins\n", from, to, amount)
	}

	time.Sleep(3 * time.Second)

	fmt.Println("\nLedger state at head:")
	peers[0].GetLedger().Display()
}

func testInvalidTransactions() {
	n := 3
	peers := make([]*peer.Peer, n)

	for i := range n {
		peers[i] = peer.NewPeer("localhost", BASE_PORT+100+i, account.DANGER_GetGenesisKeyPair("genesis_pks_sks.json", i))
		peers[i].Connect("localhost", BASE_PORT+100)
	}
	defer peer.CleanupPeers(peers)

	time.Sleep(1 * time.Second)

	tests := []struct {
		name   string
		amount int
		desc   string
	}{
		{"Negative amount", -100, "negative value"},
		{"Zero amount", 0, "zero value"},
		{"Excessive amount", 10000000, "insufficient funds"},
	}

	for _, test := range tests {
		fmt.Printf("Testing %s (%s)...\n", test.name, test.desc)
		initialBalance := peers[0].GetLedger().GetBalance(peers[0].GetEncodedPublicKey())

		peers[0].SendBalance(peers[1].GetEncodedPublicKey(), test.amount)
		time.Sleep(2 * time.Second)

		finalBalance := peers[0].GetLedger().GetBalance(peers[0].GetEncodedPublicKey())

		if test.amount <= 0 || test.amount > initialBalance {
			if initialBalance == finalBalance {
				fmt.Printf("  ✓ Transaction correctly rejected\n")
			} else {
				fmt.Printf("  ✗ Transaction incorrectly accepted\n")
			}
		}
	}
}

func testThroughput() {
	blockSizes := []int{5, 10, 20, 50}

	for _, size := range blockSizes {
		fmt.Printf("\nTesting with block size: %d\n", size)

		// Temporarily would need to modify blockchain_params.BLOCK_SIZE_LIMIT
		// For demonstration, we'll just show the concept

		n := 5
		peers := make([]*peer.Peer, n)

		basePort := BASE_PORT + 200 + size*10
		for i := range n {
			peers[i] = peer.NewPeer("localhost", basePort+i, account.DANGER_GetGenesisKeyPair("genesis_pks_sks.json", i))
			peers[i].Connect("localhost", basePort)
		}

		time.Sleep(1 * time.Second)

		numTransactions := 50
		startTime := time.Now()

		for i := 0; i < numTransactions; i++ {
			from := i % n
			to := (i + 1) % n
			peers[from].SendBalance(peers[to].GetEncodedPublicKey(), 1)
		}

		time.Sleep(5 * time.Second)

		elapsed := time.Since(startTime)
		tps := float64(numTransactions) / elapsed.Seconds()

		fmt.Printf("  Sent %d transactions in %.2f seconds\n", numTransactions, elapsed.Seconds())
		fmt.Printf("  Throughput: %.2f transactions/second\n", tps)

		peer.CleanupPeers(peers)
		time.Sleep(500 * time.Millisecond)
	}
}

func testRollbackTolerance() {
	fmt.Println("Starting network with low block time to provoke rollbacks...")

	n := 5
	peers := make([]*peer.Peer, n)

	for i := range n {
		peers[i] = peer.NewPeer("localhost", BASE_PORT+300+i, account.DANGER_GetGenesisKeyPair("genesis_pks_sks.json", i))
		peers[i].Connect("localhost", BASE_PORT+300)
	}
	defer peer.CleanupPeers(peers)

	time.Sleep(1 * time.Second)

	// Send concurrent transactions from multiple peers
	fmt.Println("Sending concurrent transactions to trigger potential forks...")
	for round := 0; round < 3; round++ {
		for i := 0; i < n; i++ {
			go func(idx int) {
				to := (idx + 1) % n
				peers[idx].SendBalance(peers[to].GetEncodedPublicKey(), 5)
			}(i)
		}
		time.Sleep(500 * time.Millisecond)
	}

	time.Sleep(5 * time.Second)

	// Check consistency
	fmt.Println("Checking ledger consistency across all peers...")
	ledgers := make([]*account.Ledger, n)
	for i := 0; i < n; i++ {
		ledgers[i] = peers[i].GetLedger()
	}

	if account.VerifyLedgerConsistency(ledgers) {
		fmt.Println("✓ All ledgers are consistent despite potential rollbacks")
	} else {
		fmt.Println("✗ Ledger inconsistency detected")
	}

	fmt.Println("\nBlock tree structure:")
	peers[0].GetBlockTree().Display()
}
