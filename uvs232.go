package uvs232

import (
	"github.com/womat/debug"
	"github.com/womat/uvs232/pkg/session"
)

// usage of parameter com: device baudrate parity databits stopbits
// eg:"/dev/ttyr00 9600 n 8 1"

// Version returns the SW Version of UVS232
func Version(com string) (v string, err error) {
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
	s := session.New()

	debug.TraceLog.Printf("open com %v", com)
	if err = s.Open(com); err != nil {
		return
	}
	defer s.Close()

	debug.TraceLog.Print("start to read current data from")
	return s.CurrentData()
}

// ReadLogger reads measurements of uvs232 data logger
func ReadLogger(com string) (m []session.Measurement, err error) {
	s := session.New()

	debug.TraceLog.Printf("open com %v", com)
	if err = s.Open(com); err != nil {
		return
	}
	defer s.Close()

	debug.TraceLog.Print("start to read data from eeprom")
	return s.ReadData()
}

// ClearLogger deletes logged measurements from uvs232 data logger
func ClearLogger(com string) (err error) {
	s := session.New()

	debug.TraceLog.Printf("open com %v", com)
	if err = s.Open(com); err != nil {
		return
	}
	defer s.Close()

	debug.TraceLog.Print("start to clear data from eeprom")
	return s.ClearData()
}
