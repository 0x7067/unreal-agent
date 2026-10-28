package attachproto

import (
	"bufio"
	"bytes"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
)

// Encode returns one deterministic JSON value followed by a newline. The size
// bound includes the newline; no conversion through interface numbers is used.
func Encode(value any) ([]byte, error) {
	data, err := json.Marshal(value, json.Deterministic(true))
	if err != nil {
		return nil, fmt.Errorf("encode attach frame: %w", err)
	}
	if len(data) >= MaxFrame {
		return nil, errors.New("attach frame exceeds maximum size")
	}
	return append(data, '\n'), nil
}

// Write writes a complete encoded frame, rejecting writers that violate the
// io.Writer contract rather than silently accepting a truncated frame.
func Write(writer io.Writer, value any) error {
	frame, err := Encode(value)
	if err != nil {
		return err
	}
	n, err := writer.Write(frame)
	if err != nil {
		return err
	}
	if n != len(frame) {
		return io.ErrShortWrite
	}
	return nil
}

// Scanner requires newline-terminated frames, including at EOF. Its bounded
// split function preserves the distinction between clean EOF and a partial tail.
func Scanner(reader io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), MaxFrame+1)
	scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		if index := bytes.IndexByte(data, '\n'); index >= 0 {
			if index+1 > MaxFrame {
				return 0, nil, errors.New("attach frame exceeds maximum size")
			}
			return index + 1, data[:index], nil
		}
		if len(data) >= MaxFrame {
			return 0, nil, errors.New("attach frame exceeds maximum size")
		}
		if atEOF && len(data) != 0 {
			return 0, nil, io.ErrUnexpectedEOF
		}
		return 0, nil, nil
	})
	return scanner
}

func readFrame(scanner *bufio.Scanner, value any) error {
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return err
		}
		return io.EOF
	}
	frame := bytes.TrimSpace(scanner.Bytes())
	if len(frame) == 0 || frame[0] != '{' {
		return errors.New("attach frame must be a JSON object")
	}
	if err := json.Unmarshal(frame, value, json.RejectUnknownMembers(true)); err != nil {
		return fmt.Errorf("decode attach frame: %w", err)
	}
	return nil
}

func ReadRequest(scanner *bufio.Scanner) (Request, error) {
	var request Request
	err := readFrame(scanner, &request)
	return request, err
}

func ReadEvent(scanner *bufio.Scanner) (Event, error) {
	var event Event
	err := readFrame(scanner, &event)
	return event, err
}
