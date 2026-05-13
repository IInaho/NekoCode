package llm

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"strings"
)

type EventReader struct {
	reader *bufio.Reader
}

func NewEventReader(r io.Reader) *EventReader {
	return &EventReader{
		reader: bufio.NewReader(r),
	}
}

func (er *EventReader) Read() (*StreamChunk, error) {
	line, err := er.reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	if len(line) < 6 || line[:6] != "data: " {
		return nil, nil
	}

	data := line[6:]
	if strings.TrimSpace(data) == "[DONE]" {
		return nil, io.EOF
	}

	var chunk StreamChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		log.Printf("Failed to parse chunk: %v, data: %s", err, data[:min(len(data), 200)])
		return nil, nil
	}

	return &chunk, nil
}
