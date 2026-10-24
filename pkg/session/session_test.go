package session

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"go.bug.st/serial"
)

// fakeLogger plays a UVS232: for each request it answers with a fixed response,
// delivered in pieces of chunk bytes, one piece per Read. Like a real port, an
// answer is appended to what is still in the input buffer.
type fakeLogger struct {
	mu        sync.Mutex
	answers   map[string][]byte // request -> response
	chunk     int
	pending   []byte
	requests  [][]byte
	closed    bool
	failSetup bool
	late      []byte // arrives shortly after each ResetInputBuffer
}

func (f *fakeLogger) Write(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, bytes.Clone(b))
	f.pending = append(f.pending, f.answers[string(b)]...)
	return len(b), nil
}

func (f *fakeLogger) Read(b []byte) (int, error) {
	f.mu.Lock()
	if len(f.pending) == 0 {
		f.mu.Unlock()
		time.Sleep(readPoll) // like a read timeout: nothing there
		return 0, nil
	}
	n := min(f.chunk, len(f.pending), len(b))
	copy(b, f.pending[:n])
	f.pending = f.pending[n:]
	f.mu.Unlock()
	return n, nil
}

func (f *fakeLogger) Close() error { f.closed = true; return nil }
func (f *fakeLogger) ResetInputBuffer() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pending = nil
	if late := f.late; late != nil {
		time.AfterFunc(sendDelay/4, func() {
			f.mu.Lock()
			f.pending = append(f.pending, late...)
			f.mu.Unlock()
		})
	}
	return nil
}

func (f *fakeLogger) ResetOutputBuffer() error           { return nil }
func (f *fakeLogger) SetDTR(bool) error                  { return nil }
func (f *fakeLogger) SetReadTimeout(time.Duration) error { return nil }
func (f *fakeLogger) SetRTS(bool) error {
	if f.failSetup {
		return errors.New("setup failed")
	}
	return nil
}

// withChecksum appends the Mod256 checksum.
func withChecksum(b ...byte) []byte {
	return append(b, checkSumMod256(b))
}

// record is one stored measurement: 4 temperatures in 0.1 °C, output byte, timestamp in 10 s.
func record(t1, t2, t3, t4 int16, outputs byte, ts uint32) []byte {
	b := make([]byte, 12)
	for i, t := range []int16{t1, t2, t3, t4} {
		binary.LittleEndian.PutUint16(b[2*i:], uint16(t))
	}
	b[8] = outputs
	b[9], b[10], b[11] = byte(ts), byte(ts>>8), byte(ts>>16)
	return b
}

func openFake(t *testing.T, f *fakeLogger) *Session {
	t.Helper()
	orig := openPort
	openPort = func(string, *serial.Mode) (port, error) { return f, nil }
	t.Cleanup(func() { openPort = orig })

	s := New()
	if err := s.Open("/dev/fake 9600 n 8 1"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestCurrentDataInPieces(t *testing.T) {
	// 11 bytes in pieces of 3: the first version read only the first piece.
	rec := record(683, 541, -12, 452, out1|5, 0)[:9]
	f := &fakeLogger{chunk: 3, answers: map[string][]byte{
		string([]byte{cmdCurrentData}): withChecksum(append([]byte{resUVR42}, rec...)...),
	}}
	s := openFake(t, f)

	m, err := s.CurrentData()
	if err != nil {
		t.Fatal(err)
	}
	if m.Temperature1 != 68.3 || m.Temperature3 != -1.2 || !m.Out1 || m.Out2 || m.RotationSpeed != 5 {
		t.Errorf("measurement %+v", m)
	}
}

func TestCurrentDataOtherDevice(t *testing.T) {
	// The first byte decides; the rest of the answer may be missing, and the
	// error must come without waiting for the timeout.
	for _, tc := range []struct {
		answer []byte
		want   string
	}{
		{[]byte{0x77, 1, 2}, UnknownDevice},
		{[]byte{resUVR31, 1, 2}, UnsupportedDevice},
	} {
		f := &fakeLogger{chunk: 11, answers: map[string][]byte{
			string([]byte{cmdCurrentData}): tc.answer,
		}}
		s := openFake(t, f)

		start := time.Now()
		_, err := s.CurrentData()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("answer % x: %v, want %q", tc.answer, err, tc.want)
		}
		if d := time.Since(start); d > timeOut/2 {
			t.Errorf("answer % x: took %v", tc.answer, d)
		}
		_ = s.Close()
	}
}

func TestLateBytesBeforeRequest(t *testing.T) {
	// Bytes of a late answer arrive shortly after the input buffer is
	// cleared; they must not end up in front of the response.
	f := &fakeLogger{chunk: 11, late: []byte{0x99}, answers: map[string][]byte{
		string([]byte{cmdReset}): {resReset},
	}}
	s := openFake(t, f)

	if err := s.ClearData(); err != nil {
		t.Fatal(err)
	}
	if len(f.requests) != 1 {
		t.Errorf("%d requests, want 1 (no retry)", len(f.requests))
	}
}

func TestVersion(t *testing.T) {
	f := &fakeLogger{chunk: 1, answers: map[string][]byte{
		string([]byte{cmdVersion}): {resUSV232_1DL},
	}}
	s := openFake(t, f)

	if v, err := s.Version(); err != nil || v != "UVS232 (1DL)" {
		t.Errorf("Version = %q, %v", v, err)
	}
}

func TestReadData(t *testing.T) {
	// Header: timestamp 1000, data from 0x10 to 0x30 = 3 records.
	header := withChecksum(resHeader, resVersion, 0xe8, 0x03, 0x00, 76, 0x10, 0x00, 0x30, 0x00)
	var block []byte
	block = append(block, record(100, 200, 300, 400, out1, 990)...)
	block = append(block, record(101, 201, 301, 401, 0, 995)...)
	block = append(block, record(102, 202, 302, 402, out2, 1000)...)

	f := &fakeLogger{chunk: 7, answers: map[string][]byte{
		string([]byte{cmdHeader}):                        header,
		string(withChecksum(cmdReadData, 0x10, 0x00, 3)): withChecksum(block...),
		string([]byte{cmdClose}):                         {resClose},
	}}
	s := openFake(t, f)

	m, err := s.ReadData()
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 3 {
		t.Fatalf("%d records", len(m))
	}
	if d := m[2].Time.Sub(m[0].Time); d != 100*time.Second {
		t.Errorf("records 0 and 2 are %v apart, want 100s", d)
	}
	if m[0].Temperature4 != 40 || !m[0].Out1 || !m[2].Out2 {
		t.Errorf("records %+v", m)
	}
}

func TestReadDataInBlocks(t *testing.T) {
	// A request reads at most 8 records; the rest follows in further requests.
	// The second case ends at the top of the address space, where an uint16
	// address would wrap around to 0 and the loop would not stop.
	for _, tc := range []struct {
		name       string
		start, end uint16
		blocks     []int // records per request
	}{
		{"10 records", 0x0010, 0x00a0, []int{8, 2}},
		{"end at 0xfff0", 0xff70, 0xfff0, []int{8, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			header := withChecksum(resHeader, resVersion, 0xe8, 0x03, 0x00, 76,
				byte(tc.start), byte(tc.start>>8), byte(tc.end), byte(tc.end>>8))
			answers := map[string][]byte{
				string([]byte{cmdHeader}): header,
				string([]byte{cmdClose}):  {resClose},
			}
			address, ts := int(tc.start), uint32(900)
			var want [][]byte
			for _, n := range tc.blocks {
				var block []byte
				for range n {
					block = append(block, record(int16(ts), 0, 0, 0, 0, ts)...)
					ts++
				}
				req := withChecksum(cmdReadData, byte(address), byte(address>>8), byte(n))
				answers[string(req)] = withChecksum(block...)
				want = append(want, req)
				address += 16 * n
			}

			f := &fakeLogger{chunk: 13, answers: answers}
			s := openFake(t, f)

			m, err := s.ReadData()
			if err != nil {
				t.Fatal(err)
			}
			total := 0
			for _, n := range tc.blocks {
				total += n
			}
			if len(m) != total {
				t.Fatalf("%d records, want %d", len(m), total)
			}
			for i := range m {
				if m[i].Temperature1 != float64(900+i)/10 {
					t.Errorf("record %d: Temperature1 %v, want %v", i, m[i].Temperature1, float64(900+i)/10)
				}
			}
			// header, the block requests, close
			got := f.requests[1 : len(f.requests)-1]
			if len(got) != len(want) {
				t.Fatalf("requests % x, want % x", got, want)
			}
			for i := range want {
				if !bytes.Equal(got[i], want[i]) {
					t.Errorf("request %d: % x, want % x", i, got[i], want[i])
				}
			}
		})
	}
}

func TestReadDataStartAfterEnd(t *testing.T) {
	// Start 0x30, end 0x10: no records would be read, and the caller would not
	// know the memory is not empty.
	header := withChecksum(resHeader, resVersion, 0xe8, 0x03, 0x00, 76, 0x30, 0x00, 0x10, 0x00)
	f := &fakeLogger{chunk: 11, answers: map[string][]byte{
		string([]byte{cmdHeader}): header,
		string([]byte{cmdClose}):  {resClose},
	}}
	s := openFake(t, f)

	if m, err := s.ReadData(); err == nil || !strings.Contains(err.Error(), "start address") {
		t.Errorf("ReadData = %d records, %v; want an error", len(m), err)
	}
}

func TestTimeoutLeavesNothingBehind(t *testing.T) {
	// No answer to the version request; the next request must get its own answer.
	f := &fakeLogger{chunk: 11, answers: map[string][]byte{
		string([]byte{cmdReset}): {resReset},
	}}
	s := openFake(t, f)

	if _, err := s.request([]byte{cmdVersion}, atLeast(1)); !errors.Is(err, errTimeOut) {
		t.Fatalf("err = %v, want time out", err)
	}
	if err := s.ClearData(); err != nil {
		t.Errorf("ClearData after a time out: %v", err)
	}
}

func TestIncompleteResponse(t *testing.T) {
	f := &fakeLogger{chunk: 4, answers: map[string][]byte{
		string([]byte{cmdHeader}): {resHeader, resVersion, 0, 0},
	}}
	s := openFake(t, f)

	if _, err := s.request([]byte{cmdHeader}, atLeast(11)); !errors.Is(err, errIncomplete) {
		t.Errorf("err = %v, want incomplete", err)
	}
}

func TestOpenCloseLock(t *testing.T) {
	f := &fakeLogger{}
	orig := openPort
	openPort = func(string, *serial.Mode) (port, error) { return f, nil }
	defer func() { openPort = orig }()

	s := New()
	if err := s.Open("/dev/fake 9600 x 8 1"); err == nil || !strings.Contains(err.Error(), "parity") {
		t.Errorf("invalid parity: %v", err)
	}

	// A failing setup closes the port and releases the lock.
	f.failSetup = true
	if err := s.Open("/dev/fake 9600 n 8 1"); err == nil || !f.closed {
		t.Fatalf("setup error: %v, port closed: %v", err, f.closed)
	}
	f.failSetup, f.closed = false, false

	if err := s.Open("/dev/fake 9600 n 8 1"); err != nil {
		t.Fatal(err)
	}
	// Closing twice must not unlock twice (that panics).
	_ = s.Close()
	_ = s.Close()

	if _, err := s.Version(); !errors.Is(err, errNotOpen) {
		t.Errorf("Version on closed session: %v", err)
	}
}
