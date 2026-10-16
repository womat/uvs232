package session

import (
	"errors"
	"log/slog"
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
	var response []byte

	if response, err = s.req([]byte{cmdVersion}, atLeast(1), "the version request has failed"); err != nil {
		return
	}

	// check Frame
	if len(response) != 1 {
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
	var response []byte

	if response, err = s.req([]byte{cmdCurrentData}, currentDataComplete, "the current data request has failed"); err != nil {
		return
	}
	n := len(response)

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
	slog.Debug("uvs232: current data", "measurement", m)

	return
}

// currentDataComplete: "no data" is one byte, an UVR42 record eleven. For the
// other controllers only the first byte matters, they are not supported.
func currentDataComplete(b []byte) bool {
	switch b[0] {
	case resUVR42:
		return len(b) >= 11
	case resNoData, resUVR31, resUVR64, resHZR65, resEEG30, resTFM66:
		return true
	}
	return false
}

// clearData deletes logged measurements from EEPROM
func (s *Session) clearData() (err error) {
	var response []byte

	if response, err = s.req([]byte{cmdReset}, atLeast(1), "the reset request has failed"); err != nil {
		return
	}

	// check Frame
	if len(response) != 1 {
		return errors.New("uvs232.clearData: " + InvalidDataLength)
	}

	// check Content
	if response[0] != resReset {
		return errors.New("uvs232.clearData: " + UnknownValue)
	}

	return
}
