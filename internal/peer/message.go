package peer

import "encoding/json"

type Cmd string

const (
	CmdAskForSetOfPeers Cmd = "ask_for_set_of_peers"
	CmdSendSetOfPeers   Cmd = "send_set_of_peers"
	CmdTransaction      Cmd = "transaction"
	CmdJoin             Cmd = "join"
)

type Message struct {
	Cmd   Cmd             `json:"cmd"`
	Flood bool            `json:"flood"`
	Data  json.RawMessage `json:"data"`
}

func NewMessage(cmd Cmd, data any) Message {
	jsonData, _ := json.Marshal(data)
	return Message{Cmd: cmd, Flood: false, Data: jsonData}
}

/*
type AnyMessage struct {
	Cmd   string          `json:"cmd"`
	Flood bool            `json:"flood"`
	Data  json.RawMessage `json:"data"`
}
*/
