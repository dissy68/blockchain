package account

import (
	"strconv"
	"testing"
)

func TestNewTransaction(t *testing.T) {
	tx := NewTransaction("tx1", "Alice", "Bob", 100)
	if tx.ID != "tx1" || tx.From != "Alice" || tx.To != "Bob" || tx.Amount != 100 {
		t.Errorf("NewTransaction() = %v, want ID=tx1, From=Alice, To=Bob, Amount=100", tx)
	}
}

func TestTransactions(t *testing.T) {
	l1 := MakeLedger()
	l2 := MakeLedger()

	for i := range 10 {
		tx1 := NewTransaction("tx1_"+strconv.Itoa(i*2), "Alice", "Bob", 100)
		tx2 := NewTransaction("tx2_"+strconv.Itoa(i*2+1), "Bob", "Alice", 20)
		l1.Transaction(tx1)
		l2.Transaction(tx1)
		l1.Transaction(tx2)
		l2.Transaction(tx2)
	}

	if !VerifyLedgerConsistency([]*Ledger{l1, l2}) {
		t.Errorf("Ledgers are inconsistent")
	}
}
