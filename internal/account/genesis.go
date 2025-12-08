package account

import (
	//blockchain_params "au_blockchain/internal"
	blockchain_params "au_blockchain/internal"
	"au_blockchain/internal/signature"
	"encoding/json"
	"os"
)

type keyEntry struct {
	Pk string `json:"pk"`
}

type keysFile struct {
	Keys []keyEntry `json:"keys"`
}

func GetInitialGenesisLedger(genesis_pks_file string) Ledger {
	data, err := os.ReadFile(genesis_pks_file)
	if err != nil {
		panic(err)
	}

	var kf keysFile
	if err := json.Unmarshal(data, &kf); err != nil {
		panic(err)
	}

	ledger := make(map[string]int, len(kf.Keys))
	for _, entry := range kf.Keys {
		ledger[entry.Pk] = blockchain_params.INITIAL_BALANCE
	}

	return Ledger{Accounts: ledger}
}

type skeyEntry struct {
	Pk string `json:"pk"`
	Sk string `json:"sk"`
}

type skeysFile struct {
	Keys []skeyEntry `json:"keys"`
}

func DANGER_GetGenesisKeyPair(genesis_pks_sks_file string, index int) *signature.KeyPair {
	data, err := os.ReadFile(genesis_pks_sks_file)
	if err != nil {
		panic(err)
	}
	var skf skeysFile
	if err := json.Unmarshal(data, &skf); err != nil {
		panic(err)
	}
	pk, err := signature.DecodePk(skf.Keys[index].Pk)
	if err != nil {
		panic(err)
	}
	sk, err := signature.DANGER_DecodeSk(skf.Keys[index].Sk)
	if err != nil {
		panic(err)
	}
	return &signature.KeyPair{
		Pk: pk,
		Sk: sk,
	}
}
