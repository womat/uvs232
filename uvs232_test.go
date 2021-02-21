package uvs232

import (
	"github.com/womat/debug"
	"github.com/womat/tools"
	"os"
	"testing"
	"time"
)

const com = "com4 9600 n 8 1"
const version = "UVS232 (1DL)"

func TestVersion(t *testing.T) {
	debug.SetDebug(os.Stderr, debug.Full)

	v, err := Version(com)

	if err != nil {
		t.Errorf("error: %q", err)
		return
	}
	if !tools.In(v, version) {
		t.Errorf("version is %q, but should be %q", v, version)
	}
}

func TestCurrentData(t *testing.T) {
	debug.SetDebug(os.Stderr, debug.Full)

	d, err := CurrentData(com)

	if err != nil {
		t.Errorf("error: %q", err)
		return
	}

	if time.Since(d.Time) > time.Second {
		t.Errorf("Timestamp is too old: %vs", time.Since(d.Time).Seconds())
		return
	}
}

func TestReadLogger(t *testing.T) {
	debug.SetDebug(os.Stderr, debug.Full)

	m, err := ReadLogger(com)

	if err != nil {
		t.Errorf("error: %q", err)
		return
	}
	if len(m) == 0 {
		t.Errorf("no data received")
		return
	}
}
