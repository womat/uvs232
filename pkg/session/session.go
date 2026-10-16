// Package session talks to a UVS232 data logger (Technische Alternative) over a
// serial interface.
package session

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"go.bug.st/serial"
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

// port is the part of serial.Port the session uses; tests replace it.
type port interface {
	io.ReadWriteCloser
	ResetInputBuffer() error
	ResetOutputBuffer() error
	SetDTR(dtr bool) error
	SetRTS(rts bool) error
	SetReadTimeout(t time.Duration) error
}

// Session is an open connection to a UVS232.
type Session struct {
	port port
	name string
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

// comPort allows one open session per process: Open locks it, Close unlocks it.
// A second Open blocks until the first session is closed.
var comPort sync.Mutex

// openPort is serial.Open; tests replace it.
var openPort = func(name string, mode *serial.Mode) (port, error) {
	return serial.Open(name, mode)
}

// New returns a closed session.
func New() (s *Session) {
	return &Session{}
}

// Open opens the serial interface, e.g. "/dev/ttyUSB0 9600 n 8 1"
// (device, baud rate, parity n/o/e/m/s, data bits, stop bits 1/1.5/2),
// and sets DTR on and RTS off, which powers the UVS232.
func (s *Session) Open(connection string) error {
	slog.Debug("uvs232: open", "connection", connection)

	var name, p, st string
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

	if _, err := fmt.Sscanf(connection, "%s %d %s %d %s", &name, &b, &p, &d, &st); err != nil {
		return fmt.Errorf("uvs232.Open: %s %q: %w", InvalidParameter, connection, err)
	}
	if _, ok := parity[p]; !ok {
		return fmt.Errorf("uvs232.Open: %s: parity %q", InvalidParameter, p)
	}
	if _, ok := stop[st]; !ok {
		return fmt.Errorf("uvs232.Open: %s: stop bits %q", InvalidParameter, st)
	}
	if s.port != nil {
		return errors.New("uvs232.Open: session is already open")
	}

	mode := &serial.Mode{
		BaudRate: b,
		Parity:   parity[p],
		DataBits: d,
		StopBits: stop[st],
	}

	comPort.Lock()

	pt, err := openPort(name, mode)
	if err == nil {
		err = setup(pt)
		if err != nil {
			_ = pt.Close()
		}
	}
	if err != nil {
		comPort.Unlock()
		return fmt.Errorf("uvs232.Open %s: %w", name, err)
	}

	s.port, s.name = pt, name
	return nil
}

func setup(p port) error {
	if err := p.SetRTS(false); err != nil {
		return err
	}
	if err := p.SetDTR(true); err != nil {
		return err
	}
	return p.SetReadTimeout(readPoll)
}

// Close closes the serial interface. Closing a closed session does nothing.
func (s *Session) Close() (err error) {
	if s.port == nil {
		return nil
	}

	slog.Debug("uvs232: close", "port", s.name)
	_ = s.port.SetDTR(false)
	_ = s.port.SetRTS(false)
	_ = s.port.ResetInputBuffer()
	_ = s.port.ResetOutputBuffer()

	err = s.port.Close()
	s.port = nil
	comPort.Unlock()
	return err
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

	if log, err = s.openLogger(); err != nil {
		return
	}

	if m, err = log.readLogger(); err != nil {
		return
	}

	return m, log.closeLogger()
}

// ClearData delete logged measurements from EEPROM
func (s *Session) ClearData() error {
	return s.clearData()
}
