package account

import "testing"

func TestVerifyLedgerConsistency(t *testing.T) {
	l1 := MakeLedger()
	l1.Accounts = map[string]int{"Alice": 100}
	l2 := MakeLedger()
	l2.Accounts = map[string]int{"Alice": 100}
	l3 := MakeLedger()
	l3.Accounts = map[string]int{"Bob": 100}

	if !VerifyLedgerConsistency([]*Ledger{l1, l2}) {
		t.Errorf("Ledgers are inconsistent")
	}

	if VerifyLedgerConsistency([]*Ledger{l1, l3}) {
		t.Errorf("Ledgers are consistent when they should be inconsistent")
	}
}
