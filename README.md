# uvs232

Go module to read a **UVS232 data logger** from Technische Alternative: the current values
of the heating controller and the measurements stored in the logger's memory.

The UVS232 sits on the controller's DL bus and talks RS-232 to the computer. It is the
manufacturer's own interface; it keeps recording while nothing is connected, and the stored
records can be fetched later. (To read the DL bus directly on a Raspberry Pi, without a
logger, see [womat/tadl](https://github.com/womat/tadl).)

Supported controller: **UVR42** (four temperature sensors, two outputs). The logger reports
UVR31, UVR64, HZR65, EEG30 and TFM66 as well; for those, `CurrentData` returns
"unsupported device".

## Usage

```bash
go get github.com/womat/uvs232
```

One command, port opened and closed around it:

```go
m, err := uvs232.CurrentData("/dev/ttyUSB0 9600 n 8 1")
```

Several commands on one open port:

```go
s := session.New() // github.com/womat/uvs232/pkg/session
if err := s.Open("/dev/ttyUSB0 9600 n 8 1"); err != nil {
    return err
}
defer s.Close()

v, err := s.Version()       // "UVS232 (1DL)", "UVS232 (2DL)", ...
m, err := s.CurrentData()   // current values
ms, err := s.ReadData()     // all records in the logger's memory
err = s.ClearData()         // empty the memory
```

The connection string is `device baud parity databits stopbits`; parity is `n`, `o`, `e`,
`m` or `s`, stop bits `1`, `1.5` or `2`. Open sets DTR on and RTS off — the UVS232 draws
its power from DTR.

Only **one session per process** can be open; a second `Open` blocks until the first
session is closed.

## Measurements

```go
type Measurement struct {
    Time          time.Time
    Temperature1  float64 // °C, sensors 1-4
    Temperature2  float64
    Temperature3  float64
    Temperature4  float64
    Out1          bool    // outputs 1 and 2
    Out2          bool
    RotationSpeed float64 // speed step of output 1
}
```

The logger stores a 10-second counter, not a clock time. `ReadData` converts it relative
to the moment the memory is read; records come back oldest first. A record older than its
predecessor is logged as a warning and still returned — filter it if that matters.

## Errors, timeouts, retries

Every command waits up to 1 s for a complete answer and is repeated once after 500 ms if
it fails. An answer is read until it has the length the command expects, so a response
that arrives in several pieces (at 9600 baud a full data block takes about 100 ms) is put
back together. Nothing keeps reading after a command has returned, and the input buffer is
cleared right before each command is sent, so a late answer to an earlier command is not
taken for the answer to the next one.

## Logging

The module logs through `log/slog` with the default logger: the exchanged bytes at debug
level, retries at warning level. Set the default logger in your program to control it.

## Tests

```bash
go test -race ./...
```

The tests in `pkg/session` need no hardware; they play the data logger and deliver its
answers in small pieces. The tests in the module root talk to a real device and are skipped
unless `UVS232_PORT` is set:

```bash
UVS232_PORT="/dev/ttyUSB0 9600 n 8 1" go test .
```

## Changes in v0.1.0

- Logging through `log/slog`; `github.com/womat/debug` is gone (the module no longer
  exists, so v0.0.9 cannot be built any more).
- Serial interface through `go.bug.st/serial` instead of `github.com/albenik/go-serial`.
- Responses are read until complete; a timeout no longer leaves a goroutine reading the
  port. `Session` no longer embeds the serial port.
- `Close` can be called twice; a failing `Open` closes the port and releases the lock.
- No endless loop for memory addresses near 0xFFFF; no wrap-around for a record newer than
  the header.
