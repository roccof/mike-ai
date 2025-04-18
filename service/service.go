package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/coder/websocket"
)

type WSService struct {
	ws *websocket.Conn
}

func NewWSService(ws *websocket.Conn) *WSService {
	return &WSService{ws: ws}
}

func (s *WSService) Close() {
	s.ws.Close(websocket.StatusNormalClosure, "")
}

func (s *WSService) SendMessage(ctx context.Context, msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	err = s.ws.Write(ctx, websocket.MessageText, data)
	if err != nil {
		return err
	}

	return nil
}

func parseMessage[T Message](data []byte) (T, error) {
	var t T
	err := json.Unmarshal(data, &t)
	if err != nil {
		return t, err
	}
	return t, nil
}

func (s *WSService) ReadMessage(ctx context.Context) (Message, error) {
	type MessageTypeEvent struct {
		Type MessageType `json:"type"`
	}
	var msgType MessageTypeEvent

	_, msg, err := s.ws.Read(ctx)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(msg, &msgType)
	if err != nil {
		return nil, err
	}

	switch msgType.Type {
	case MessageTypeAudio:
		return parseMessage[AudioMessage](msg)

	case MessageTypeCommandResult:
		return parseMessage[CommandResultMessage](msg)

	default:
		return nil, fmt.Errorf("unknown message type: %s", msgType.Type)
	}
}
