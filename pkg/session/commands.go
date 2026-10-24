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
	resUVS232_1DL = 0xA7
	resUVS232_2DL = 0xD0
	resNoData     = 0xAB
	resUVR31      = 0x30
	resUVR42      = 0x10
	resUVR64      = 0x20
	resHZR65      = 0x60
	resEEG30      = 0x50
	resTFM66      = 0x40
	resHeader     = 0x0F
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
	case resUVS232_1DL:
		return "UVS232 (1DL)", nil
	case resUVS232_2DL:
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

	if n == 0 {
		return m, errors.New("uvs232.CurrentData: " + NoDataReceived)
	}

	// check Content; the checksum only for the record that is used, the
	// response of another controller may be cut off after the first byte
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
		if !checkMod256(response) {
			return m, errors.New("uvs232.CurrentData: " + InvalidChecksum)
		}
		m = getMeasurement(response[1:10])
	default:
		return m, errors.New("uvs232.CurrentData: " + UnknownDevice)
	}

	m.Time = time.Now()
	slog.Debug("uvs232: current data", "measurement", m)

	return
}

// currentDataComplete: an UVR42 record is eleven bytes. For anything else the
// first byte decides - "no data", an unsupported or an unknown device - so
// there is no need to wait for the timeout.
func currentDataComplete(b []byte) bool {
	if b[0] == resUVR42 {
		return len(b) >= 11
	}
	return true
}

// clearData deletes logged measurements from EEPROM
func (s *Session) clearData() (err error) {
	var response []byte

	if response, err = s.req([]byte{cmdReset}, atLeast(1), "the reset request has failed"); err != nil {
		return
	}

	// check Frame
	if len(response) != 1 {
		return errors.New("uvs232.ClearData: " + InvalidDataLength)
	}

	// check Content
	if response[0] != resReset {
		return errors.New("uvs232.ClearData: " + UnknownValue)
	}

	return
}
