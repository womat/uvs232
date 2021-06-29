package session

import (
	"encoding/binary"
	"errors"
	"github.com/womat/debug"
	"io"
	"time"
)

type logger struct {
	*Session
	timeStamp    uint32
	Time         time.Time
	recordLength uint16
	startAddress uint16
	endAddress   uint16
}

// openLogger reads the header of uvs232 data block
func (s *Session) openLogger() (log *logger, err error) {
	var n int

	response := make([]byte, maxBufferSize)
	send := []byte{cmdHeader}

	if n, err = s.req(send, response, "the request to read a data header has failed"); err != nil {
		return
	}

	// check Frame
	switch {
	case n == 0:
		return nil, io.EOF
	case n != 11:
		return nil, errors.New("uvs232.openLog: " + InvalidDataLength)
	case !checkMod256(response[:n]):
		return nil, errors.New("uvs232.openLog: " + InvalidChecksum)
	}

	// Check Content
	switch {
	case response[0] != resHeader:
		return nil, errors.New("uvs232.openLog: " + InvalidIdentifier)
	case response[1] != resVersion:
		return nil, errors.New("uvs232.openLog: " + InvalidVersion)
	}

	log = &logger{
		Session:      s,
		timeStamp:    convertTimeStamp(response[2:5]),
		Time:         time.Now().Truncate(time.Second),
		recordLength: uint16(response[5]) - 64,
		startAddress: binary.LittleEndian.Uint16(response[6:8]),
		endAddress:   binary.LittleEndian.Uint16(response[8:10]),
	}

	debug.DebugLog.Printf("Time: %v, TimeStamp: %v, RecordLength: %v, StartAddress: %x, EndAddress: %x [% x]", log.Time, log.timeStamp, log.recordLength, log.startAddress, log.endAddress, response[:n])
	return log, nil
}

// readLogger reads a usv232 data block
func (log *logger) readLogger() (measurements []Measurement, err error) {
	// no data available
	if log.startAddress < 0x10 || log.endAddress <= 0x10 {
		return make([]Measurement, 0), nil
	}

	measurements = make([]Measurement, 0, 2000)

	const nrOfFramesMax = 8
	nrOfFrames := nrOfFramesMax

	for address := log.startAddress; address <= log.endAddress; address += 16 * nrOfFramesMax {
		if address+16*nrOfFramesMax > log.endAddress {
			nrOfFrames = int((log.endAddress-address)/16 + 1)
		}

		send := []byte{cmdReadData, 0, 0, byte(nrOfFrames), 0}
		binary.LittleEndian.PutUint16(send[1:3], address)
		send[4] = checkSumMod256(send[:4])

		var n int
		response := make([]byte, maxBufferSize)

		if n, err = log.req(send, response, "the request to read a data record has failed"); err != nil {
			return measurements, err
		}

		// check Frame
		switch {
		case n < 2:
			return measurements, errors.New("uvs232.readLog: " + InvalidDataLength)
		case !checkMod256(response[:n]):
			return measurements, errors.New("uvs232.readLog: " + InvalidChecksum)
		}

		// Check Content
		if nrOfFrames*12+1 != n {
			return measurements, errors.New("uvs232.readLog: " + InvalidDataLength)
		}

		for i := 0; i < nrOfFrames; i++ {
			idx := i * 12
			data := getMeasurement(response[idx : idx+9])
			timeStamp := convertTimeStamp(response[idx+9 : idx+12])
			data.Time = log.Time.Add(time.Duration(log.timeStamp-timeStamp) * -10 * time.Second)

			if i > 0 && data.Time.Before(measurements[i-1].Time) {
				lastTimeStamp := measurements[i].Time.Sub(log.Time).Seconds()/10 + float64(log.timeStamp)
				debug.ErrorLog.Printf("timestamp %v is older than last %v  %v [% x]", timeStamp, lastTimeStamp, data, response[idx:idx+12])
			} else {
				debug.DebugLog.Printf("timestamp %v %v [% x]", timeStamp, data, response[idx:idx+12])
			}

			measurements = append(measurements, data)
		}
	}

	return
}

// closeLogger reads footer of uvs232 data block
func (log *logger) closeLogger() (err error) {
	var n int

	response := make([]byte, maxBufferSize)
	send := []byte{cmdClose}

	if n, err = log.req(send, response, "the request to read the end of data has failed"); err != nil {
		return
	}

	// check Frame
	if n != 1 {
		return errors.New("uvs232.closeLog: " + InvalidDataLength)
	}

	// check Content
	if response[0] != resClose {
		return errors.New("uvs232.closeLog: " + UnknownValue)
	}

	return
}
