package deej

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jacobsa/go-serial/serial"
	"go.uber.org/zap"

	"github.com/omriharel/deej/pkg/deej/util"
)

const (
	// while the board is gone, deej tries to open its port again this often
	reconnectInterval = 2 * time.Second

	// a board that has been talking steadily (chattyLineCount lines in a row, each within
	// chattyLineGap of the last) and then goes quiet for silenceTimeout counts as gone, for
	// drivers that keep the port open after the cable is pulled. deej-5-sliders-oled sends at
	// least once a second and the vanilla firmware constantly; a firmware that only sends on
	// change never looks steady, so it's never dropped for being quiet
	chattyLineGap   = 2 * time.Second
	chattyLineCount = 5
	silenceTimeout  = 5 * time.Second
)

// SerialIO provides a deej-aware abstraction layer to managing serial I/O
type SerialIO struct {
	comPort  string
	baudRate uint

	deej   *Deej
	logger *zap.SugaredLogger

	connected   bool
	connOptions serial.OpenOptions
	conn        io.ReadWriteCloser

	// the connection loop (see maintain): running while connected or waiting to reconnect
	lifeLock sync.Mutex
	running  bool
	quit     chan struct{} // closed by Stop
	loopDone chan struct{} // closed when the loop has exited

	// a failed write reports here, so the connection loop can reconnect (one per connection)
	connLost chan error

	lastKnownNumSliders        int
	currentSliderPercentValues []float32

	sliderMoveConsumers []chan SliderMoveEvent

	// 1 when the board should get display info (labels) with the next slider line, see display.go
	displayPending int32
	writeLock      sync.Mutex

	// sliders the board reported since this connection opened; 0 until its first line arrives
	displaySliders int32

	// closed when this connection closes, to stop its display keepalive
	connDone chan struct{}
}

// SliderMoveEvent represents a single slider move captured by deej
type SliderMoveEvent struct {
	SliderID     int
	PercentValue float32
}

var expectedLinePattern = regexp.MustCompile(`^\d{1,4}(\|\d{1,4})*\r\n$`)

// NewSerialIO creates a SerialIO instance that uses the provided deej
// instance's connection info to establish communications with the arduino chip
func NewSerialIO(deej *Deej, logger *zap.SugaredLogger) (*SerialIO, error) {
	logger = logger.Named("serial")

	sio := &SerialIO{
		deej:                deej,
		logger:              logger,
		connected:           false,
		conn:                nil,
		sliderMoveConsumers: []chan SliderMoveEvent{},
	}

	logger.Debug("Created serial i/o instance")

	// respond to config changes
	sio.setupOnConfigReload()

	return sio, nil
}

// Start attempts to connect to our arduino chip. If it succeeds, deej stays connected from then
// on: when the board goes away, it keeps trying to reconnect until Stop is called. If it fails,
// nothing is left running; call StartReconnecting to wait for the board instead.
func (sio *SerialIO) Start() error {
	sio.lifeLock.Lock()
	defer sio.lifeLock.Unlock()

	// don't allow multiple concurrent connections
	if sio.running {
		sio.logger.Warn("Already connected, can't start another without closing first")
		return errors.New("serial: connection already active")
	}

	conn, err := sio.open()
	if err != nil {
		sio.logger.Warnw("Failed to open serial connection", "error", err)
		return err
	}

	sio.startLoop(conn)
	return nil
}

// StartReconnecting waits for the board in the background: it tries to open the port every
// reconnectInterval, and once connected behaves like Start. For when the board isn't there yet.
func (sio *SerialIO) StartReconnecting() {
	sio.lifeLock.Lock()
	defer sio.lifeLock.Unlock()

	if sio.running {
		return
	}

	sio.logger.Infow("Waiting for the board",
		"comPort", sio.deej.config.ConnectionInfo.COMPort,
		"retryEvery", reconnectInterval)
	sio.startLoop(nil)
}

// startLoop runs the connection loop, starting from conn (or from waiting, if conn is nil).
// lifeLock must be held
func (sio *SerialIO) startLoop(conn io.ReadWriteCloser) {
	sio.running = true
	sio.quit = make(chan struct{})
	sio.loopDone = make(chan struct{})
	go sio.maintain(conn, sio.quit, sio.loopDone)
}

// maintain serves a connection until it's lost, then reopens the port every reconnectInterval
// until it's back, over and over, until quit is closed.
func (sio *SerialIO) maintain(conn io.ReadWriteCloser, quit <-chan struct{}, done chan<- struct{}) {
	defer close(done)

	waiting := false
	for {
		if conn != nil {
			err := sio.serve(conn, quit)
			if err == nil {
				return // stopped
			}

			sio.logger.Warnw("Lost the connection to the board, will reconnect when it's back", "error", err)
			conn = nil
			waiting = true
		}

		select {
		case <-quit:
			return
		case <-time.After(reconnectInterval):
		}

		var err error
		if conn, err = sio.open(); err != nil {
			// every couple of seconds while the board is unplugged, or while something else
			// (like the Arduino IDE flashing it) has the port: not worth more than a debug line
			sio.logger.Debugw("Board not available yet", "error", err)
			continue
		}

		if waiting {
			sio.logger.Info("The board is back, reconnected")
		}
	}
}

// open opens the port from the current config
func (sio *SerialIO) open() (io.ReadWriteCloser, error) {

	// set minimum read size according to platform (0 for windows, 1 for linux)
	// this prevents a rare bug on windows where serial reads get congested,
	// resulting in significant lag
	minimumReadSize := 0
	if util.Linux() {
		minimumReadSize = 1
	}

	sio.connOptions = serial.OpenOptions{
		PortName:        sio.deej.config.ConnectionInfo.COMPort,
		BaudRate:        uint(sio.deej.config.ConnectionInfo.BaudRate),
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: uint(minimumReadSize),
	}

	sio.logger.Debugw("Attempting serial connection",
		"comPort", sio.connOptions.PortName,
		"baudRate", sio.connOptions.BaudRate,
		"minReadSize", minimumReadSize)

	conn, err := serial.Open(sio.connOptions)
	if err != nil {
		return nil, fmt.Errorf("open serial connection: %w", err)
	}

	return conn, nil
}

// serve reads lines from conn until it's lost (returns why) or quit is closed (returns nil).
// Either way, conn is closed when it returns.
func (sio *SerialIO) serve(conn io.ReadWriteCloser, quit <-chan struct{}) error {
	logger := sio.logger.Named(strings.ToLower(sio.connOptions.PortName))
	logger.Infow("Connected", "conn", conn)

	connDone := make(chan struct{})
	connLost := make(chan error, 1)

	sio.writeLock.Lock()
	sio.conn = conn
	sio.connected = true
	sio.connDone = connDone
	sio.connLost = connLost
	sio.writeLock.Unlock()

	// once the board talks, send every slider's volume again: after a reconnect, the knobs may
	// have moved while it was unplugged
	sio.lastKnownNumSliders = 0

	// the board may reset when the port opens, so wait for its first line before sending labels
	atomic.StoreInt32(&sio.displayPending, 1)
	atomic.StoreInt32(&sio.displaySliders, 0)

	// tell the board every few seconds that deej is alive and which apps are running
	go sio.runDisplayKeepalive(logger, connDone)

	lines := sio.readLine(logger, bufio.NewReader(&emptyReadRetrier{r: conn, done: connDone}), connDone)

	silenceCheck := time.NewTicker(time.Second)
	defer silenceCheck.Stop()

	var lastLine time.Time
	steadyLines := 0

	for {
		select {
		case <-quit:
			sio.close(logger)
			return nil

		case err := <-connLost:
			sio.close(logger)
			return err

		case line, ok := <-lines:
			if !ok {
				sio.close(logger)
				return errors.New("reading from the port failed")
			}

			now := time.Now()
			if !lastLine.IsZero() && now.Sub(lastLine) < chattyLineGap {
				steadyLines++
			} else {
				steadyLines = 0
			}
			lastLine = now

			sio.handleLine(logger, line)

		case now := <-silenceCheck.C:
			if steadyLines >= chattyLineCount && now.Sub(lastLine) > silenceTimeout {
				sio.close(logger)
				return fmt.Errorf("the board stopped sending (nothing for %s)", silenceTimeout)
			}
		}
	}
}

// Stop shuts down our serial connection, or stops waiting for the board, and waits until the
// port is closed
func (sio *SerialIO) Stop() {
	sio.lifeLock.Lock()
	if !sio.running {
		sio.lifeLock.Unlock()
		sio.logger.Debug("Not currently connected, nothing to stop")
		return
	}

	sio.logger.Debug("Shutting down serial connection")
	sio.running = false
	close(sio.quit)
	done := sio.loopDone
	sio.lifeLock.Unlock()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		sio.logger.Warn("Serial connection didn't close in time")
	}
}

// SubscribeToSliderMoveEvents returns an unbuffered channel that receives
// a sliderMoveEvent struct every time a slider moves
func (sio *SerialIO) SubscribeToSliderMoveEvents() chan SliderMoveEvent {
	ch := make(chan SliderMoveEvent)
	sio.sliderMoveConsumers = append(sio.sliderMoveConsumers, ch)

	return ch
}

func (sio *SerialIO) setupOnConfigReload() {
	configReloadedChannel := sio.deej.config.SubscribeToChanges()

	const stopDelay = 50 * time.Millisecond

	go func() {
		for {
			select {
			case <-configReloadedChannel:

				// make any config reload unset our slider number to ensure process volumes are being re-set
				// (the next read line will emit SliderMoveEvent instances for all sliders)\
				// this needs to happen after a small delay, because the session map will also re-acquire sessions
				// whenever the config file is reloaded, and we don't want it to receive these move events while the map
				// is still cleared. this is kind of ugly, but shouldn't cause any issues
				go func() {
					<-time.After(stopDelay)
					sio.lastKnownNumSliders = 0

					// labels may have changed too: resend them with the next slider line
					atomic.StoreInt32(&sio.displayPending, 1)
				}()

				// if connection params have changed, attempt to stop and start the connection
				if sio.deej.config.ConnectionInfo.COMPort != sio.connOptions.PortName ||
					uint(sio.deej.config.ConnectionInfo.BaudRate) != sio.connOptions.BaudRate {

					sio.logger.Info("Detected change in connection parameters, attempting to renew connection")
					sio.Stop() // waits for the old port to close

					if err := sio.Start(); err != nil {
						sio.logger.Warnw("Failed to renew connection after parameter change, will keep trying", "error", err)
						sio.StartReconnecting()
					} else {
						sio.logger.Debug("Renewed connection successfully")
					}
				}
			}
		}
	}()
}

func (sio *SerialIO) close(logger *zap.SugaredLogger) {
	sio.writeLock.Lock()
	defer sio.writeLock.Unlock()

	if sio.connDone != nil {
		close(sio.connDone)
		sio.connDone = nil
	}

	if sio.conn != nil {
		if err := sio.conn.Close(); err != nil {
			logger.Warnw("Failed to close serial connection", "error", err)
		} else {
			logger.Debug("Serial connection closed")
		}
	}

	sio.conn = nil
	sio.connected = false
	sio.connLost = nil
}

// readLine reads lines on its own goroutine and delivers them on the returned channel, which is
// closed when a read fails. It stops when done is closed.
func (sio *SerialIO) readLine(logger *zap.SugaredLogger, reader *bufio.Reader, done <-chan struct{}) <-chan string {
	ch := make(chan string)

	go func() {
		defer close(ch)

		for {
			line, err := reader.ReadString('\n')
			if err != nil {

				if sio.deej.Verbose() {
					logger.Warnw("Failed to read line from serial", "error", err, "line", line)
				}

				// the connection loop sees the closed channel and reconnects
				return
			}

			if sio.deej.Verbose() {
				logger.Debugw("Read new line", "line", line)
			}

			// deliver the line to the channel
			select {
			case ch <- line:
			case <-done:
				return
			}
		}
	}()

	return ch
}

func (sio *SerialIO) handleLine(logger *zap.SugaredLogger, line string) {

	// this function receives an unsanitized line which is guaranteed to end with LF,
	// but most lines will end with CRLF. it may also have garbage instead of
	// deej-formatted values, so we must check for that! just ignore bad ones
	if !expectedLinePattern.MatchString(line) {
		return
	}

	// trim the suffix
	line = strings.TrimSuffix(line, "\r\n")

	// split on pipe (|), this gives a slice of numerical strings between "0" and "1023"
	splitLine := strings.Split(line, "|")
	numSliders := len(splitLine)

	// update our slider count, if needed - this will send slider move events for all
	if numSliders != sio.lastKnownNumSliders {
		logger.Infow("Detected sliders", "amount", numSliders)
		sio.lastKnownNumSliders = numSliders
		sio.currentSliderPercentValues = make([]float32, numSliders)

		// reset everything to be an impossible value to force the slider move event later
		for idx := range sio.currentSliderPercentValues {
			sio.currentSliderPercentValues[idx] = -1.0
		}
	}

	// for each slider:
	moveEvents := []SliderMoveEvent{}
	for sliderIdx, stringValue := range splitLine {

		// convert string values to integers ("1023" -> 1023)
		number, _ := strconv.Atoi(stringValue)

		// turns out the first line could come out dirty sometimes (i.e. "4558|925|41|643|220")
		// so let's check the first number for correctness just in case
		if sliderIdx == 0 && number > 1023 {
			sio.logger.Debugw("Got malformed line from serial, ignoring", "line", line)
			return
		}

		// map the value from raw to a "dirty" float between 0 and 1 (e.g. 0.15451...)
		dirtyFloat := float32(number) / 1023.0

		// normalize it to an actual volume scalar between 0.0 and 1.0 with 2 points of precision
		normalizedScalar := util.NormalizeScalar(dirtyFloat)

		// if sliders are inverted, take the complement of 1.0
		if sio.deej.config.InvertSliders {
			normalizedScalar = 1 - normalizedScalar
		}

		// check if it changes the desired state (could just be a jumpy raw slider value)
		if util.SignificantlyDifferent(sio.currentSliderPercentValues[sliderIdx], normalizedScalar, sio.deej.config.NoiseReductionLevel) {

			// if it does, update the saved value and create a move event
			sio.currentSliderPercentValues[sliderIdx] = normalizedScalar

			moveEvents = append(moveEvents, SliderMoveEvent{
				SliderID:     sliderIdx,
				PercentValue: normalizedScalar,
			})

			if sio.deej.Verbose() {
				logger.Debugw("Slider moved", "event", moveEvents[len(moveEvents)-1])
			}
		}
	}

	// the board is up and talking: send it the labels, if it hasn't had them since (re)connecting
	atomic.StoreInt32(&sio.displaySliders, int32(numSliders))
	if atomic.CompareAndSwapInt32(&sio.displayPending, 1, 0) {
		go sio.sendDisplayInfo(logger, numSliders)
	}

	// deliver move events if there are any, towards all potential consumers
	if len(moveEvents) > 0 {
		for _, consumer := range sio.sliderMoveConsumers {
			for _, moveEvent := range moveEvents {
				consumer <- moveEvent
			}
		}
	}
}

// emptyReadRetrier waits out reads that return no data and no error. Newer CH340 drivers on
// Windows (3.8 / 3.9, 2023 onwards) do that between bytes instead of blocking, and bufio gives
// up after 100 of them in a row ("multiple Read calls return no data or error"), which ended
// the connection right after it opened. Older drivers never return empty, so nothing changes there
type emptyReadRetrier struct {
	r    io.Reader
	done <-chan struct{} // the connection closed: stop waiting
}

func (e *emptyReadRetrier) Read(p []byte) (int, error) {
	for {
		n, err := e.r.Read(p)
		if n > 0 || err != nil || len(p) == 0 {
			return n, err
		}

		select {
		case <-e.done:
			return 0, io.EOF
		case <-time.After(5 * time.Millisecond):
		}
	}
}
