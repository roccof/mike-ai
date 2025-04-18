package service

type Message interface {
	GetType() MessageType
}

type MessageType string

const (
	MessageTypeAudio         MessageType = "audio"
	MessageTypeCommandInvoke MessageType = "command.invoke"
	MessageTypeCommandResult MessageType = "command.result"
	MessageTypeError         MessageType = "error"
)

type BaseMessage struct {
	Type MessageType `json:"type"`
}

func (m BaseMessage) GetType() MessageType {
	return m.Type
}

type AudioMessage struct {
	BaseMessage
	Audio string `json:"audio"`
}

type CommandInvokeMessage struct {
	BaseMessage
	Name   string `json:"name"`
	CallID string `json:"call_id"`
	Args   string `json:"args"`
}

type CommandResultMessage struct {
	BaseMessage
	Name   string `json:"name"`
	CallID string `json:"call_id"`
	Output string `json:"output"`
}
