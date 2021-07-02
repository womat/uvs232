package uvs232

import (
	"github.com/womat/debug"
	"github.com/womat/uvs232/pkg/session"
)

// usage of parameter com: device baudrate parity databits stopbits
// eg:"/dev/ttyr00 9600 n 8 1"

// Version returns the SW Version of UVS232
func Version(com string) (v string, err error) {
	debug.TraceLog.Print("start Version()")
	s := session.New()

	if err = s.Open(com); err != nil {
		return
	}
	defer s.Close()

	return s.Version()
}

// CurrentData reads the current measurements
// e.g. CurrentData("/dev/ttyr00 9600 n 8 1")
func CurrentData(com string) (m session.Measurement, err error) {
	debug.TraceLog.Print("start CurrentData()")
	s := session.New()

	if err = s.Open(com); err != nil {
		return
	}
	defer s.Close()

	return s.CurrentData()
}

// ReadData reads measurements of uvs232 data logger
func ReadData(com string) (m []session.Measurement, err error) {
	debug.TraceLog.Print("start ReadLogger()")
	s := session.New()

	if err = s.Open(com); err != nil {
		return
	}
	defer s.Close()

	return s.ReadData()
}

// ClearData deletes logged measurements from uvs232 data logger
func ClearData(com string) (err error) {
	debug.TraceLog.Print("start ClearData)=")
	s := session.New()

	if err = s.Open(com); err != nil {
		return
	}
	defer s.Close()

	return s.ClearData()
}
