package blockchain

import (
	"au_blockchain/internal/signature"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"strconv"
	"time"
)

const LOTTERY_PREFIX = "lottery"

const SlotLength = 1 // in seconds

func Hash(seed string, slot int, pk *signature.PublicKey, draw *big.Int) *big.Int {
	h := sha256.New()
	h.Write([]byte(seed))

	var slotBuf [8]byte
	binary.BigEndian.PutUint64(slotBuf[:], uint64(slot))
	h.Write(slotBuf[:])

	h.Write([]byte(pk.Encode()))

	h.Write(draw.Bytes())

	sum := h.Sum(nil)
	hashInt := new(big.Int).SetBytes(sum)
	if hashInt.Sign() < 0 {
		hashInt.Neg(hashInt)
	}

	return hashInt
	/*
			zeros := 0
			// Check how many bits of least significance are zero
			for hashInt.Bit(zeros) == 0 {
				zeros++
			}
		return big.NewInt(int64(zeros))
	*/
}

func HashValue(hashInt *big.Int) *big.Int {
	zeros := 0
	// Check how many bits of least significance are zero
	for hashInt.Bit(zeros) == 0 {
		zeros++
	}
	return big.NewInt(int64(zeros))
}

func Draw(slot int, keyPair *signature.KeyPair) *big.Int {
	draw := signature.Sign([]byte(LOTTERY_PREFIX+strconv.Itoa(slot)), keyPair.Sk)
	return draw
}

func VerifyDraw(slot int, pk *signature.PublicKey, draw *big.Int) bool {
	expectedPrefix := LOTTERY_PREFIX + strconv.Itoa(slot)
	verified := signature.Verify([]byte(expectedPrefix), draw, pk)
	return verified
}

var GenesisTime = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

func CurrentSlot(now time.Time) int {
	if !now.After(GenesisTime) {
		return 0
	}
	elapsed := now.Sub(GenesisTime)
	return int(elapsed / (SlotLength * time.Second))
}
