package atserial

import (
	"io"
	"log"
	"sync"
	"time"
	"errors"
	"strings"

	"go.bug.st/serial"
)

type SerialRequest struct {
	ID uint32
	Data []byte
	Timeout time.Duration
}

type SerialResponse struct {
	ID uint32
	Data []byte
	Err error
}

type msgIn struct {
	req SerialRequest
	ch chan SerialResponse
}

type PortDaemon struct {
	portname string
	baudrate int

	reqChan chan msgIn
	txChan chan []byte
	rxChan chan []byte

	quit chan struct{}
}

func StartPortDaemon(portname string, baudrate int) (*PortDaemon, error) {
	
	mode := &serial.Mode {
		BaudRate: baudrate,
	}
	port, err := serial.Open(portname, mode)
	port.ResetInputBuffer()
	port.ResetOutputBuffer()
	
	if err != nil {
		return nil, err
	}

	pd := &PortDaemon {
		portname: portname,
		baudrate: baudrate,
		reqChan: make(chan msgIn, 64),
		txChan: make(chan []byte, 1024),
		rxChan: make(chan []byte, 1024),
		quit: make(chan struct{}),
	}

	go pd.readLoop(port)
	go pd.writeLoop(port)
	go pd.run()

	return pd, nil
}

func (pd *PortDaemon) Query(req SerialRequest) (SerialResponse, error) {

	reply := make(chan SerialResponse, 1)
	
	select {
	case pd.reqChan <- msgIn{req: req, ch: reply}:

	case <- time.After(req.Timeout):
		return SerialResponse{}, errors.New("serial daemon channel full/timeout")
	}

	select {
	case rsp := <- reply:
		return rsp, nil

	case <- time.After(req.Timeout):
		return SerialResponse{}, errors.New("serial daemon response timeout")
	}
}

func (pd *PortDaemon) readLoop(port serial.Port) {
	
	defer port.Close()
	buf := make([]byte, 1024)
	
	for {
		n, err := port.Read(buf)
		if err != nil {
			pd.rxChan <- nil
			return 
		}
		tmp := make([]byte, n)
		copy(tmp, buf[:n])
		pd.rxChan <- tmp
	}
}

func (pd *PortDaemon) writeLoop(port serial.Port) {

	for {
		select {
		case b := <- pd.txChan:
			if _, err := port.Write(b); err != nil {
				return
			}
			
		case <- pd.quit:
			return
		}
	}
}

func (pd *PortDaemon) run() {

	pending := make(map[uint32]chan SerialResponse)

	for {
		select {
		case m := <- pd.reqChan:
			pd.txChan <- m.req.Data
			pending[m.req.ID] = m.ch

		case data := <- pd.rxChan:
			if data == nil { 
				for _, ch := range pending {
					ch <- SerialResponse{Err: io.ErrClosedPipe}
				}
				return
			}
			if len(pending) == 0 {
				continue
			}
			
			var id uint32
			var ch chan SerialResponse
			for id, ch = range pending {
				break
			}

			var responseData []byte
			responseData = append(responseData, data...)
			for {
				select {
				case moreData := <- pd.rxChan:
					if moreData == nil {
						ch <- SerialResponse{ID: id, Data: responseData, Err: io.ErrClosedPipe}
						delete(pending, id)
						continue
					}

					responseData = append(responseData, moreData...)
					responseStr := string(responseData)

					if strings.Contains(responseStr, "OK") || strings.Contains(responseStr, "ERROR") {
						delete(pending, id)
						ch <- SerialResponse{ID: id, Data: responseData}
						break
					}

				case <- time.After(500 * time.Millisecond):
					delete(pending, id)
					ch <- SerialResponse{ID: id, Data: responseData}

				case <- pd.quit:
					return
				}	
			}
			
			
		case <- pd.quit:
			return
		}
	}
}

func (pd *PortDaemon) Stop() {
	close(pd.quit)
}

const restartInterval = 3 * time.Second

type SerialSupervisor struct {
	portname string
	baudrate int

	mu sync.RWMutex
	daemon *PortDaemon
	started chan struct{}
}

func NewSupervisor(portname string, baudrate int) *SerialSupervisor {

	s := &SerialSupervisor{
		portname: portname,
		baudrate: baudrate,
		started: make(chan struct{}),
	}
	go s.supervisor()

	return s
}

func (s *SerialSupervisor) supervisor() {

	for {
		d, err := StartPortDaemon(s.portname, s.baudrate)
		if err != nil {
			log.Println("[SerialSupervisor] start daemon failed:", err)
			time.Sleep(restartInterval)
		}

		s.mu.Lock()
		s.daemon = d
		s.mu.Unlock()

		select {
		case <- s.started:

		default:
			close(s.started)
		}

		done := make(chan struct{})
		go func() {
			d.run()
			close(done)
		} ()
		<- done

		s.mu.Lock()
		s.daemon = nil
		s.mu.Unlock()

		log.Println("[SerialSupervisor] daemon down, restart later")
		time.Sleep(restartInterval)
	}
}

func (s *SerialSupervisor) Query(req SerialRequest) (SerialResponse, error) {

	<- s.started

	s.mu.RLock()
	d := s.daemon
	s.mu.RUnlock()

	if d == nil {
		return SerialResponse{}, errors.New("no available daemon")
	}
	return d.Query(req)
}

