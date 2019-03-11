package common

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
)

type Message struct {
	HeadLen uint16
	BodyLen uint16
	Head    map[string]interface{}
	Body    []byte
}

func NewMessage() *Message {
	return &Message{}
}

func (msg *Message) IsHeartbeat() bool {
	return len(msg.Body) == 0
}

func (msg *Message) CloneBody(header map[string]interface{}) (*Message, error) {
	newMsg := NewMessage()
	headData, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}
	newMsg.HeadLen = uint16(len(headData))
	newMsg.Head = header
	newMsg.BodyLen = msg.BodyLen
	newMsg.Body = make([]byte, msg.BodyLen)
	copy(newMsg.Body, msg.Body)

	return newMsg, nil
}

func (msg *Message) Encode() ([]byte, error) {
	buffer := new(bytes.Buffer)
	headData, err := json.Marshal(msg.Head)
	err = binary.Write(buffer, binary.BigEndian, uint16(len(headData)))
	if err != nil {
		return nil, err
	}

	err = binary.Write(buffer, binary.BigEndian, uint16(len(msg.Body)))
	if err != nil {
		return nil, err
	}
	err = binary.Write(buffer, binary.BigEndian, headData)
	if err != nil {
		return nil, err
	}
	err = binary.Write(buffer, binary.BigEndian, msg.Body)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func (msg *Message) Decode(reader io.Reader) error {
	buf := make([]byte, 4)
	_, err := io.ReadFull(reader, buf)

	if err != nil {
		return err
	}

	headLen := binary.BigEndian.Uint16(buf[:2])
	bodyLen := binary.BigEndian.Uint16(buf[2:])

	headData, err := readBuf(headLen, reader)
	if err != nil {
		return err
	}

	var head map[string]interface{}

	err = json.Unmarshal(headData, &head)
	if err != nil {
		return err
	}

	bodyData, err := readBuf(bodyLen, reader)
	if err != nil {
		return err
	}
	msg.HeadLen = headLen
	msg.BodyLen = bodyLen
	msg.Head = head
	msg.Body = bodyData

	return nil
}

func readBuf(length uint16, reader io.Reader) ([]byte, error) {
	buf := make([]byte, length)
	_, err := io.ReadFull(reader, buf)
	if err != nil {
		return nil, err
	}

	bufReader := bytes.NewReader(buf)
	b := make([]byte, length)
	err = binary.Read(bufReader, binary.BigEndian, &b)
	if err != nil {
		return nil, err
	}

	return b, nil
}
