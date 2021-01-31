package uvs232

import (
	"github.com/womat/tools"
	"testing"
	"time"
)

const com = "com4 9600 n 8 1"
const version = "UVS232 (1DL)"

func TestVersion(t *testing.T) {
	v, err := Version(com)

	if err != nil {
		t.Errorf("error: %q", err)
	}
	if !tools.In(v, version) {
		t.Errorf("version is %q, but should be %q", v, version)
	}
}

func TestCurrentData(t *testing.T) {
	d, err := CurrentData(com)

	if err != nil {
		t.Errorf("error: %q", err)
	}

	if time.Since(d.Time) > time.Second {
		t.Errorf("Timestamp is too old: %vs", time.Since(d.Time).Seconds())
	}
}

func TestReadLogger(t *testing.T) {
	m, err := ReadLogger(com)

	if err != nil {
		t.Errorf("error: %q", err)
	}
	if len(m) == 0 {
		t.Errorf("no data received")
	}
}
