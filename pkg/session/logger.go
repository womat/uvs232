package session

import (
	"encoding/binary"
	"errors"
	"log/slog"
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
	var response []byte

	if response, err = s.req([]byte{cmdHeader}, atLeast(11), "the request to read a data header has failed"); err != nil {
		return
	}
	n := len(response)

	// check Frame
	switch {
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

	slog.Debug("uvs232: logger header", "time", log.Time, "timestamp", log.timeStamp, "recordLength", log.recordLength,
		"startAddress", log.startAddress, "endAddress", log.endAddress)
	return log, nil
}

// readLogger reads a usv232 data block
func (log *logger) readLogger() (measurements []Measurement, err error) {
	var lastTimestamp time.Time

	// no data available
	if log.startAddress < 0x10 || log.endAddress <= 0x10 {
		return make([]Measurement, 0), nil
	}

	measurements = make([]Measurement, 0, 2000)

	const nrOfFramesMax = 8
	start, end := int(log.startAddress), int(log.endAddress)

	// int, not uint16: with an end address near 0xFFFF, address += 128 would
	// wrap around and the loop would never end.
	for address := start; address <= end; address += 16 * nrOfFramesMax {
		nrOfFrames := nrOfFramesMax
		if address+16*nrOfFramesMax > end {
			nrOfFrames = (end-address)/16 + 1
		}

		send := []byte{cmdReadData, 0, 0, byte(nrOfFrames), 0}
		binary.LittleEndian.PutUint16(send[1:3], uint16(address))
		send[4] = checkSumMod256(send[:4])

		var response []byte
		if response, err = log.req(send, atLeast(nrOfFrames*12+1), "the request to read a data record has failed"); err != nil {
			return measurements, err
		}
		n := len(response)

		// check Frame
		switch {
		case n < 2:
			return measurements, errors.New("uvs232.readLog: " + InvalidDataLength)
		case nrOfFrames*12+1 != n:
			return measurements, errors.New("uvs232.readLog: " + InvalidDataLength)
		case !checkMod256(response):
			return measurements, errors.New("uvs232.readLog: " + InvalidChecksum)
		}

		for i := 0; i < nrOfFrames; i++ {
			idx := i * 12
			data := getMeasurement(response[idx : idx+9])
			timestamp := convertTimeStamp(response[idx+9 : idx+12])
			// signed: a record newer than the header must not wrap into the far past
			age := int64(log.timeStamp) - int64(timestamp)
			data.Time = log.Time.Add(time.Duration(age) * -10 * time.Second)

			if data.Time.Before(lastTimestamp) {
				slog.Warn("uvs232: record is older than the previous one", "timestamp", timestamp, "time", data.Time, "previous", lastTimestamp)
			} else {
				lastTimestamp = data.Time
			}

			measurements = append(measurements, data)
		}
	}

	return
}

// closeLogger reads footer of uvs232 data block
func (log *logger) closeLogger() (err error) {
	var response []byte

	if response, err = log.req([]byte{cmdClose}, atLeast(1), "the request to read the end of data has failed"); err != nil {
		return
	}

	// check Frame
	if len(response) != 1 {
		return errors.New("uvs232.closeLog: " + InvalidDataLength)
	}

	// check Content
	if response[0] != resClose {
		return errors.New("uvs232.closeLog: " + UnknownValue)
	}

	return
}
