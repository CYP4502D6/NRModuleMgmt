package atserial

import (
	"io"
	"log"
	"sync"
	"time"
	"bytes"
	"errors"
//	"strings"

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

type pendingQueue struct {
	sync.Mutex
	q []msgIn
}

func (pq *pendingQueue) push(m msgIn) {
	log.Println("[PortQueue] push msg", m.req.ID)
	pq.Lock()
	pq.q = append(pq.q, m)
	pq.Unlock()
}

func (pq *pendingQueue) headCh() (chan SerialResponse, bool) {
	pq.Lock()
	defer pq.Unlock()

	if len(pq.q) == 0 {
		return nil, false
	}
	//log.Println("[PortQueue] head ch is", pq.q[0].req.ID)
	return pq.q[0].ch, true
}

func (pq *pendingQueue) pop() {
	pq.Lock()
	if len(pq.q) > 0 {
		log.Println("[PortQueue] queue:")
		log.Println(pq.q)
		pq.q = pq.q[1:]
	}
	pq.Unlock()
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
	buf := make([]byte, 32768)
	
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

func isStatusStop(p []byte) bool {
	if bytes.Contains(p, []byte("OK")) || bytes.Contains(p, []byte("ERROR")) || bytes.Contains(p, []byte(">")){
		return true
	}
	return false
}

func (pd *PortDaemon) waitFullResp(resp []byte, rxChan <-chan []byte, quit <-chan struct{}) ([]byte, error) {
	for {
		if isStatusStop(resp) {
			return resp, nil
		}
		select {
		case more := <-rxChan:
			if more == nil {
				return resp, io.ErrClosedPipe
			}
			resp = append(resp, more...)

		case <-time.After(500 * time.Millisecond):
			log.Println("[PortDaemon] response timeout without status")
			return resp, nil

		case <-quit:
			return nil, errors.New("daemon quitting")
		}
	}
}

func (pd *PortDaemon) run() {
	pending := &pendingQueue{}
	
	for {
		select {
		case m := <-pd.reqChan:
			pd.txChan <- m.req.Data
			pending.push(m)
						
		case data := <-pd.rxChan:
			if data == nil {
				pending.Lock()
				for _, v := range pending.q {
					v.ch <- SerialResponse{Err: io.ErrClosedPipe}
				}
				pending.q = nil
				pending.Unlock()
				return
			}
			
			ch, ok := pending.headCh()
			if !ok {
				log.Println("[PortDaemon] receive no id data", string(data))
				continue
			}

			full, err := pd.waitFullResp(data, pd.rxChan, pd.quit)
			if err != nil {
				ch <- SerialResponse{Err: err}
			} else {
				ch <- SerialResponse{ID: 0, Data: full}
			}
			pending.pop()

		case <-pd.quit:
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

