// Package uvs232 reads a Technische Alternative UVS232 data logger: the
// controller's current values and the measurements stored in its memory.
//
// The functions here open the serial interface, run one command and close it
// again. To run several commands, use a session from pkg/session.
package uvs232

import (
	"log/slog"

	"github.com/womat/uvs232/pkg/session"
)

// usage of parameter com: device baudrate parity databits stopbits
// eg:"/dev/ttyr00 9600 n 8 1"

// Version returns the SW Version of UVS232
func Version(com string) (v string, err error) {
	slog.Debug("uvs232: Version")
	s := session.New()

	if err = s.Open(com); err != nil {
		return
	}
	defer func() { _ = s.Close() }()

	return s.Version()
}

// CurrentData reads the current measurements
// e.g. CurrentData("/dev/ttyr00 9600 n 8 1")
func CurrentData(com string) (m session.Measurement, err error) {
	slog.Debug("uvs232: CurrentData")
	s := session.New()

	if err = s.Open(com); err != nil {
		return
	}
	defer func() { _ = s.Close() }()

	return s.CurrentData()
}

// ReadData reads measurements of uvs232 data logger
func ReadData(com string) (m []session.Measurement, err error) {
	slog.Debug("uvs232: ReadData")
	s := session.New()

	if err = s.Open(com); err != nil {
		return
	}
	defer func() { _ = s.Close() }()

	return s.ReadData()
}

// ClearData deletes logged measurements from uvs232 data logger
func ClearData(com string) (err error) {
	slog.Debug("uvs232: ClearData")
	s := session.New()

	if err = s.Open(com); err != nil {
		return
	}
	defer func() { _ = s.Close() }()

	return s.ClearData()
}
