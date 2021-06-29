package session

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/albenik/go-serial"
	"github.com/womat/debug"
)

const (
	InvalidChecksum   = "invalid checksum"
	InvalidDataLength = "wrong response data length"
	InvalidIdentifier = "wrong response identifier"
	InvalidParameter  = "invalid parameter"
	InvalidVersion    = "wrong version"
	NoDataAvailable   = "no data available, sync is running"
	NoDataReceived    = "no data received"
	UnknownDevice     = "unknown device"
	UnknownValue      = "unknown response value"
	UnsupportedDevice = "unsupported device"
)

// Session is the interface for a serial Port
type Session struct {
	serial.Port
}

// Measurement is the measured data
type Measurement struct {
	Time          time.Time
	Temperature1  float64
	Temperature2  float64
	Temperature3  float64
	Temperature4  float64
	Out1          bool
	Out2          bool
	RotationSpeed float64
}

// comPort prevents to open serial device twice
var comPort sync.Mutex

// New generates a serial device handler and set DTR ON and RTS off
func New() (s *Session) {
	return &Session{}
}

// Open generates a serial device handler and set DTR ON and RTS off
func (s *Session) Open(connection string) (err error) {
	var port, p, st string
	var b, d int

	parity := map[string]serial.Parity{
		"n": serial.NoParity,
		"o": serial.OddParity,
		"e": serial.EvenParity,
		"m": serial.MarkParity,
		"s": serial.SpaceParity,
	}

	stop := map[string]serial.StopBits{
		"1":   serial.OneStopBit,
		"1.5": serial.OnePointFiveStopBits,
		"2":   serial.TwoStopBits,
	}

	if _, err = fmt.Sscanf(connection, "%s %d %s %d %s", &port, &b, &p, &d, &st); err != nil {
		return
	}
	if _, ok := parity[p]; !ok {
		return errors.New("uvs232.Open: " + InvalidParameter)
	}
	if _, ok := stop[st]; !ok {
		return errors.New("uvs232.Open: " + InvalidParameter)
	}

	mode := &serial.Mode{
		BaudRate: b,
		Parity:   parity[p],
		DataBits: d,
		StopBits: stop[st],
	}

	// ComPort will be unlocked with the Close() function
	debug.TraceLog.Print("lock the com port")
	comPort.Lock()
	debug.TraceLog.Print("com port is locked")

	func() {
		if s.Port, err = serial.Open(port, mode); err != nil {
			return
		}
		if err = s.Port.SetRTS(false); err != nil {
			return
		}
		if err = s.Port.SetDTR(true); err != nil {
			return
		}
	}()

	if err != nil {
		// in the event of an error, the comPort must never remain blocked!
		comPort.Unlock()
	}

	return
}

// Close closes the serial device
func (s *Session) Close() (err error) {
	if s.Port != nil {
		_ = s.Port.SetDTR(false)
		_ = s.Port.SetRTS(false)

		_ = s.Port.ResetInputBuffer()
		_ = s.Port.ResetOutputBuffer()

		err = s.Port.Close()
		comPort.Unlock()
		debug.TraceLog.Print("com port is unlocked")
	}

	return
}

// Version returns the SW Version of UVS232
func (s *Session) Version() (version string, err error) {
	return s.version()
}

// CurrentData reads the current measurements
func (s *Session) CurrentData() (m Measurement, err error) {
	return s.currentData()
}

// ReadData reads the logged Data from EEPROM
func (s *Session) ReadData() (m []Measurement, err error) {
	var log *logger

	debug.TraceLog.Print("start to read data header")
	if log, err = s.openLogger(); err != nil {
		return
	}

	debug.TraceLog.Print("start to read data block")
	if m, err = log.readLogger(); err != nil {
		return
	}

	debug.TraceLog.Print("start to read data footer")
	return m, log.closeLogger()
}

// ClearData delete logged measurements from EEPROM
func (s *Session) ClearData() error {
	return s.clearData()
}
