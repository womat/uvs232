package session

import (
	"errors"
	"github.com/womat/debug"
	"time"
)

const (
	maxBufferSize = 128
	retryTime     = 500 * time.Millisecond
)

const (
	cmdVersion     = 0x81
	cmdCurrentData = 0xAB
	cmdHeader      = 0xAA
	cmdReadData    = 0xAC
	cmdClose       = 0xAD
	cmdReset       = 0xAF
)

const (
	out1 = 1 << 5
	out2 = out1 << 1
	rota = 0x1f
)

const (
	resOldVersion = 0xA6
	resUSV232_1DL = 0xA7
	resUSV232_2DL = 0xD0
	resNoData     = 0xAB
	resUVR31      = 0x30
	resUVR42      = 0x10
	resUVR64      = 0x20
	resHZR65      = 0x60
	resEEG30      = 0x50
	resTFM66      = 0x40
	resHeader     = 0x0F
	resVersion    = 0xA7
	resClose      = 0xAD
	resReset      = 0xAF
)

// version returns the SW Version of UVS232
func (s *Session) version() (version string, err error) {
	var n int

	response := make([]byte, maxBufferSize)
	send := []byte{cmdVersion}

	if n, err = s.req(send, response, "the version request has failed"); err != nil {
		return
	}

	// check Frame
	if n != 1 {
		return "", errors.New("uvs232.Version: " + InvalidDataLength)
	}

	// check Content
	switch response[0] {
	case resOldVersion:
		return "old version (update required)", nil
	case resUSV232_1DL:
		return "UVS232 (1DL)", nil
	case resUSV232_2DL:
		return "UVS232 (2DL)", nil
	}

	return "", errors.New("uvs232.Version: " + UnknownValue)
}

// currentData reads the current measurements
func (s *Session) currentData() (m Measurement, err error) {
	var n int

	response := make([]byte, maxBufferSize)
	send := []byte{cmdCurrentData}

	if n, err = s.req(send, response, "the current data request has failed"); err != nil {
		return
	}

	// check Frame
	switch {
	case n == 0:
		return m, errors.New("uvs232.CurrentData: " + NoDataReceived)
	case n > 1 && !checkMod256(response[:n]):
		return m, errors.New("uvs232.CurrentData: " + InvalidChecksum)
	}

	// check Content
	switch response[0] {
	case resNoData:
		if n != 1 {
			return m, errors.New("uvs232.CurrentData: " + InvalidDataLength)
		}
		return m, errors.New("uvs232.CurrentData: " + NoDataAvailable)
	case resUVR31,
		resUVR64,
		resHZR65,
		resEEG30,
		resTFM66:
		return m, errors.New("uvs232.CurrentData: " + UnsupportedDevice)
	case resUVR42:
		if n != 11 {
			return m, errors.New("uvs232.CurrentData: " + InvalidDataLength)
		}
		m = getMeasurement(response[1:10])
	default:
		return m, errors.New("uvs232.CurrentData: " + UnknownDevice)
	}

	m.Time = time.Now()
	debug.DebugLog.Printf("measurement %v [% x]", m, response[1:10])

	return
}

// resetLog deletes logged measurements from EEPROM
func (s *Session) clearData() (err error) {
	var n int

	response := make([]byte, maxBufferSize)
	send := []byte{cmdReset}

	if n, err = s.req(send, response, "the reset request has failed"); err != nil {
		return
	}

	// check Frame
	if n != 1 {
		return errors.New("uvs232.clearData: " + InvalidDataLength)
	}

	// check Content
	if response[0] != resReset {
		return errors.New("uvs232.clearData: " + UnknownValue)
	}

	return
}
