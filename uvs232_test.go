package uvs232

import (
	"os"
	"testing"
	"time"
)

// These tests need a UVS232 at a real serial interface, e.g.
//
//	UVS232_PORT="/dev/ttyUSB0 9600 n 8 1" go test .
//
// Without UVS232_PORT they are skipped.
func connection(t *testing.T) string {
	t.Helper()
	c := os.Getenv("UVS232_PORT")
	if c == "" {
		t.Skip("UVS232_PORT not set, no device to test against")
	}
	return c
}

func TestDeviceVersion(t *testing.T) {
	v, err := Version(connection(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("version %q", v)
}

func TestDeviceCurrentData(t *testing.T) {
	d, err := CurrentData(connection(t))
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(d.Time) > time.Second {
		t.Errorf("timestamp is %v old", time.Since(d.Time))
	}
}

func TestDeviceReadData(t *testing.T) {
	m, err := ReadData(connection(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d records", len(m))
}
