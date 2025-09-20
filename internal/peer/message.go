package peer

import (
	"encoding/json"

	"github.com/google/uuid"
)

type Cmd string

const (
	CmdAskForSetOfPeers Cmd = "ask_for_set_of_peers"
	CmdSetOfPeers       Cmd = "set_of_peers"
	CmdMessageHistory   Cmd = "message_history"
	CmdTransaction      Cmd = "transaction"
	CmdJoin             Cmd = "join"
)

type Message struct {
	Id    string          `json:"id"`
	Cmd   Cmd             `json:"cmd"`
	Flood bool            `json:"flood"`
	Data  json.RawMessage `json:"data"`
}

func NewMessage(cmd Cmd, data any) Message {
	jsonData, _ := json.Marshal(data)
	id := uuid.New().String()
	return Message{Id: id, Cmd: cmd, Flood: false, Data: jsonData}
}
