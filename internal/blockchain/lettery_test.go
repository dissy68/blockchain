package blockchain

import (
	"au_blockchain/internal/signature"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"testing"
)

// --- Tests for Hash ---

func TestHashMatchesManualComputation(t *testing.T) {
	seed := "test-seed"
	slot := 42
	pk := signature.NewPublicKey(big.NewInt(123456789), big.NewInt(3))
	draw := big.NewInt(987654321)

	got := Hash(seed, slot, pk, draw)

	// Manual computation of expected hash (mirrors Hash implementation)
	h := sha256.New()
	h.Write([]byte(seed))

	var slotBuf [8]byte
	binary.BigEndian.PutUint64(slotBuf[:], uint64(slot))
	h.Write(slotBuf[:])

	h.Write([]byte(pk.Encode()))
	h.Write(draw.Bytes())

	sum := h.Sum(nil)
	expected := new(big.Int).SetBytes(sum)

	if got.Cmp(expected) != 0 {
		t.Fatalf("Hash mismatch: expected %s, got %s", expected.String(), got.String())
	}
}

func TestHashDeterministic(t *testing.T) {
	seed := "same-seed"
	slot := 7
	pk := signature.NewPublicKey(big.NewInt(11111), big.NewInt(3))
	draw := big.NewInt(22222)

	h1 := Hash(seed, slot, pk, draw)
	h2 := Hash(seed, slot, pk, draw)

	if h1.Cmp(h2) != 0 {
		t.Fatalf("Hash is not deterministic: %s != %s", h1.String(), h2.String())
	}
}

func TestHashSensitiveToInputs(t *testing.T) {
	seed := "base-seed"
	slot := 10
	pk := signature.NewPublicKey(big.NewInt(33333), big.NewInt(3))
	draw := big.NewInt(44444)

	base := Hash(seed, slot, pk, draw)

	changedSeed := Hash("other-seed", slot, pk, draw)
	if base.Cmp(changedSeed) == 0 {
		t.Error("expected different hash when seed changes")
	}

	changedSlot := Hash(seed, slot+1, pk, draw)
	if base.Cmp(changedSlot) == 0 {
		t.Error("expected different hash when slot changes")
	}

	otherPk := signature.NewPublicKey(big.NewInt(55555), big.NewInt(3))
	changedPk := Hash(seed, slot, otherPk, draw)
	if base.Cmp(changedPk) == 0 {
		t.Error("expected different hash when public key changes")
	}

	changedDraw := Hash(seed, slot, pk, big.NewInt(44445))
	if base.Cmp(changedDraw) == 0 {
		t.Error("expected different hash when draw changes")
	}
}

// --- Tests for Draw & VerifyDraw ---

func TestDrawAndVerifyRoundTrip(t *testing.T) {
	kp := signature.DefaultKeyGen()
	slot := 5

	draw := Draw(slot, kp)
	if draw == nil {
		t.Fatal("Draw returned nil")
	}

	if !VerifyDraw(slot, kp.Pk, draw) {
		t.Fatalf("VerifyDraw should succeed for draw produced by Draw with same key and slot")
	}
}

func TestVerifyDrawFailsOnWrongInputs(t *testing.T) {
	kp := signature.DefaultKeyGen()
	slot := 12

	draw := Draw(slot, kp)
	if draw == nil {
		t.Fatal("Draw returned nil")
	}

	// Sanity: correct case
	if !VerifyDraw(slot, kp.Pk, draw) {
		t.Fatalf("VerifyDraw should succeed for valid draw")
	}

	t.Run("wrong slot", func(t *testing.T) {
		if VerifyDraw(slot+1, kp.Pk, draw) {
			t.Fatalf("VerifyDraw should fail when slot is wrong")
		}
	})

	t.Run("wrong public key", func(t *testing.T) {
		otherKP := signature.DefaultKeyGen()
		if VerifyDraw(slot, otherKP.Pk, draw) {
			t.Fatalf("VerifyDraw should fail when public key is wrong")
		}
	})

	t.Run("tampered draw", func(t *testing.T) {
		// Create a modified copy of the draw
		tampered := new(big.Int).Add(draw, big.NewInt(1))
		if VerifyDraw(slot, kp.Pk, tampered) {
			t.Fatalf("VerifyDraw should fail for a tampered draw")
		}
	})
}
