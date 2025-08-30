package peer

import "encoding/json"

type Message struct {
	Cmd   string          `json:"cmd"`
	Flood bool            `json:"flood"`
	Data  json.RawMessage `json:"data"`
}

func MakeMessage(cmd string, data any) Message {
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
