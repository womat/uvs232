package session

import (
	"encoding/binary"
	"errors"
	"io"
	"time"

	"github.com/womat/debug"
)

const (
	// delay between two requests
	sendDelay = 20 * time.Millisecond
	// timeout to receive a response
	timeOut = 1 * time.Second
	// Error Message for time out
	errTimeOut = "time out"
)

// request received uvs232 logger Data from the serial interface
func (s *Session) request(request []byte, response []byte) (n int, err error) {
	// clear input/output buffer
	if err = s.Port.ResetInputBuffer(); err != nil {
		return
	}
	if err = s.Port.ResetOutputBuffer(); err != nil {
		return
	}

	time.Sleep(sendDelay)
	start := time.Now()
	debug.TraceLog.Printf("request: [% x]\n", request)
	if _, err = s.Port.Write(request); err != nil {
		debug.TraceLog.Printf("error to write serial interface: %v\n", err)
		return
	}

	//	buffer = make([]byte, maxBufferSize)
	done := make(chan bool, 1)

	go func() {
		if n, err = s.Port.Read(response); n == 0 {
			debug.TraceLog.Printf("error to read serial interface: %v\n", err)
			err = io.EOF
		}
		debug.TraceLog.Printf("response (%v bytes): [% x]\n", n, response[:n])
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeOut):
		err = errors.New(errTimeOut)
		debug.TraceLog.Printf("error to read serial interface: %v\n", err)
		return
	}

	debug.TraceLog.Printf("request runtime: %vms\n", time.Since(start).Milliseconds())
	return
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
	if len(buffer) != 9 {
		return
	}

	m.Temperature1 = float64(int16(binary.LittleEndian.Uint16(buffer[0:2]))) / 10
	m.Temperature2 = float64(int16(binary.LittleEndian.Uint16(buffer[2:4]))) / 10
	m.Temperature3 = float64(int16(binary.LittleEndian.Uint16(buffer[4:6]))) / 10
	m.Temperature4 = float64(int16(binary.LittleEndian.Uint16(buffer[6:8]))) / 10
	m.Out1 = buffer[8]&out1 > 0
	m.Out2 = buffer[8]&out2 > 0
	m.RotationSpeed = float64(buffer[8] & rota)
	return
}

// req send and receives a response with error handling
func (s *Session) req(send, response []byte, errorMsg string) (n int, err error) {
	if n, err = s.request(send, response); err != nil {
		debug.WarningLog.Printf("%v, the request will be executed again in %vms second", errorMsg, retryTime.Milliseconds())
		time.Sleep(retryTime)

		if n, err = s.request(send, response); err != nil {
			debug.ErrorLog.Print(errorMsg)
		}
	}
	return
}
