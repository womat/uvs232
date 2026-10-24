package session

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

const (
	// delay between two requests
	sendDelay = 20 * time.Millisecond
	// timeout to receive a complete response
	timeOut = 1 * time.Second
	// readPoll bounds a single Read, so the deadline is checked regularly
	readPoll = 20 * time.Millisecond
)

var (
	errNotOpen    = errors.New("session is not open")
	errTimeOut    = errors.New("time out")
	errIncomplete = errors.New("incomplete response")
)

// atLeast reports a response as complete once it has n bytes.
func atLeast(n int) func([]byte) bool {
	return func(b []byte) bool { return len(b) >= n }
}

// request sends a command and reads until complete reports the response as
// whole or the timeout expires.
//
// A response can arrive in several pieces - at 9600 baud a data block of 97
// bytes takes about 100 ms, and a USB adapter delivers it in chunks - so one
// Read is not enough. Reading happens synchronously: nothing is left reading
// the port after request returns, so a late answer cannot be taken for the
// answer to the next command.
func (s *Session) request(send []byte, complete func([]byte) bool) ([]byte, error) {
	if s.port == nil {
		return nil, errNotOpen
	}

	// Wait first, then clear: bytes of a late answer that arrive during the
	// delay must not be taken for the start of this response.
	time.Sleep(sendDelay)
	if err := s.port.ResetInputBuffer(); err != nil {
		return nil, err
	}
	if err := s.port.ResetOutputBuffer(); err != nil {
		return nil, err
	}

	start := time.Now()
	slog.Debug("uvs232: send request", "data", hexBytes(send))
	if _, err := s.port.Write(send); err != nil {
		return nil, err
	}

	deadline := start.Add(timeOut)
	response := make([]byte, 0, maxBufferSize)
	chunk := make([]byte, maxBufferSize)
	for {
		n, err := s.port.Read(chunk)
		if err != nil {
			return response, err
		}
		response = append(response, chunk[:n]...)

		if len(response) > 0 && complete(response) {
			slog.Debug("uvs232: receive response", "bytes", len(response), "data", hexBytes(response), "runtime", time.Since(start))
			return response, nil
		}
		if len(response) >= maxBufferSize || time.Now().After(deadline) {
			break
		}
	}

	if len(response) == 0 {
		return nil, errTimeOut
	}
	slog.Debug("uvs232: incomplete response", "bytes", len(response), "data", hexBytes(response))
	return response, errIncomplete
}

// checkMod256 check Mod256 checksum of the last data byte
func checkMod256(buffer []byte) bool {
	return buffer[len(buffer)-1] == checkSumMod256(buffer[:len(buffer)-1])
}

// checkSumMod256 calculate a Mod256 Checksum
func checkSumMod256(buffer []byte) (chkSum byte) {
	for _, b := range buffer {
		chkSum += b
	}

	return
}

// convertTimeStamp read time stamp
func convertTimeStamp(response []byte) (timeStamp uint32) {
	timeStamp = binary.LittleEndian.Uint32([]byte{response[0], response[1], response[2], 0})
	return
}

// getMeasurement reads a data record
func getMeasurement(buffer []byte) (m Measurement) {
	m.Temperature1 = float64(int16(binary.LittleEndian.Uint16(buffer[0:2]))) / 10
	m.Temperature2 = float64(int16(binary.LittleEndian.Uint16(buffer[2:4]))) / 10
	m.Temperature3 = float64(int16(binary.LittleEndian.Uint16(buffer[4:6]))) / 10
	m.Temperature4 = float64(int16(binary.LittleEndian.Uint16(buffer[6:8]))) / 10
	m.Out1 = buffer[8]&out1 > 0
	m.Out2 = buffer[8]&out2 > 0
	m.RotationSpeed = float64(buffer[8] & rota)
	return
}

// req sends a request and repeats it once after retryTime if it fails.
func (s *Session) req(send []byte, complete func([]byte) bool, errorMsg string) ([]byte, error) {
	response, err := s.request(send, complete)
	if err == nil {
		return response, nil
	}
	if errors.Is(err, errNotOpen) {
		return nil, err
	}

	slog.Warn("uvs232: "+errorMsg+", retrying", "error", err, "retry", retryTime)
	time.Sleep(retryTime)

	if response, err = s.request(send, complete); err != nil {
		return response, fmt.Errorf("%s: %w", errorMsg, err)
	}
	return response, nil
}

// hexBytes formats bytes for the log only when the line is written.
type hexBytes []byte

func (h hexBytes) LogValue() slog.Value {
	const digits = "0123456789abcdef"
	b := make([]byte, 0, 3*len(h))
	for i, c := range h {
		if i > 0 {
			b = append(b, ' ')
		}
		b = append(b, digits[c>>4], digits[c&0x0f])
	}
	return slog.StringValue(string(b))
}
