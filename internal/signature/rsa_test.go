package signature

import (
	"fmt"
	"testing"
)

func TestSignature(t *testing.T) {
	kp := DefaultKeyGen()
	message := []byte("Hello, World!")
	signature := Sign(message, kp.Sk)
	if !Verify(message, signature, kp.Pk) {
		t.Errorf("Failed to verify signature")
	}
}

func TestLongSignature(t *testing.T) {
	kp := KeyGen(DEFAULT_KEY_SIZE * 2)
	message := []byte("This is a longer message to test longer RSA signature length on longer message.")
	signature := Sign(message, kp.Sk)
	if !Verify(message, signature, kp.Pk) {
		t.Errorf("Failed to verify signature")
	}
}

func TestGetOneSignatureStrings(t *testing.T) {
	// Print the pk and sk as strings
	// Print it 10 times in json format with "pk": "...", "sk": "..."
	// Print with only 1 Logf
	for i := 0; i < 10; i++ {
		kp := DefaultKeyGen()
		pkStr := kp.Pk.Encode()
		skStr := kp.Sk.DANGER_Encode()
		fmt.Printf("{\n    \"pk\": \"%s\",\n    \"sk\": \"%s\"\n}\n", pkStr, skStr)
	}
}
