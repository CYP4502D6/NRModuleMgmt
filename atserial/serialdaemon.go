package atserial

import (
	"log"
	"sync"
	"time"
	"bytes"
	"errors"
	"strings"
	"context"
	"encoding/hex"
	"crypto/sha256"

	"go.bug.st/serial"
)

type SerialRequest struct {
	ID      uint32
	Data    []byte
	Timeout time.Duration
}

type SerialResponse struct {
	ID   uint32
	Data []byte
	Err  error
}

type msgIn struct {
	req SerialRequest
	ch  chan SerialResponse
}


type cacheEntry struct {
	response  []byte
	timestamp time.Time
	ttl       time.Duration
}

type inFlightRequest struct {
	ch   chan SerialResponse
	done chan struct{}
}

type PortDaemon struct {
	portname string
	baudrate int
	port     serial.Port

	reqChan chan msgIn
	quit    chan struct{}
	mu      sync.Mutex
	running bool
	cmdMu   sync.Mutex

	activeRequests sync.Map

	cacheMutex sync.RWMutex
	cmdCache   map[string]cacheEntry

	inFlightMutex sync.Mutex
	inFlight      map[string]*inFlightRequest
}

func (pd *PortDaemon) initializePort() {

	log.Println("[PortDaemon] Initializing port...")
	
	if pd.port == nil {
		log.Println("[PortDaemon] Port is nil, cannot initialize")
		return
	}
	
	mode := &serial.Mode{
        BaudRate: pd.baudrate,
        DataBits: 8,
        Parity:   serial.NoParity,
        StopBits: serial.OneStopBit,
    }

    if err := pd.port.SetMode(mode); err != nil {
        log.Printf("[PortDaemon] Failed to set mode: %v", err)
    }
    
	if err := pd.port.SetDTR(true); err != nil {
        log.Printf("[PortDaemon] Failed to set DTR: %v", err)
    }
	
    if err := pd.port.SetRTS(true); err != nil {
        log.Printf("[PortDaemon] Failed to set RTS: %v", err)
    }
    
	if _, err := pd.port.Write([]byte("ATE0\r\n")); err != nil {
		log.Printf("[PortDaemon] Failed to send ATE0: %v", err)
	}
	
	time.Sleep(50 * time.Millisecond) 

	buf := make([]byte, 4096)
	pd.port.SetReadTimeout(time.Second * 2)
	if n, err := pd.port.Read(buf); err != nil {
		log.Printf("[PortDaemon] Error reading ATE0 response: %v", err)
	} else {
		log.Printf("[PortDaemon] ATE0 response: %s", string(buf[:n]))
	}
	
	if _, err := pd.port.Write([]byte("AT+QURCCFG=\"urcport\",\"uart1\"\r\n")); err != nil {
		log.Printf("[PortDaemon] Failed to send QURCCFG: %v", err)
	}
	
	if n, err := pd.port.Read(buf); err != nil {
		log.Printf("[PortDaemon] Error reading QURCCFG response: %v", err)
	} else {
		log.Printf("[PortDaemon] QURCCFG response: %s", string(buf[:n]))
	}
	

	pd.port.SetReadTimeout(time.Millisecond * 100)

	
    pd.port.Write([]byte("AT\r\n"))
    syncBuf := make([]byte, 128)
    pd.port.Read(syncBuf)
	log.Println("[PortDaemon] Read sync buffer:", string(syncBuf))
	
	log.Println("[PortDaemon] Port ready and waiting for requests.")
}


func StartPortDaemon(portname string, baudrate int) (*PortDaemon, error) {
	
	mode := &serial.Mode{BaudRate: baudrate}
	port, err := serial.Open(portname, mode)
	if err != nil {
		log.Println("[PortDaemon] failed to open port:", err)
		return nil, err
	}

	pd := &PortDaemon{
		portname: portname,
		baudrate: baudrate,
		port:     port,
		reqChan:  make(chan msgIn, 10),
		quit:     make(chan struct{}),
		running:  true,
		cmdCache: make(map[string]cacheEntry),
		inFlight: make(map[string]*inFlightRequest),
	}

	pd.initializePort()
	go pd.run()
	go pd.cacheCleaner()

	return pd, nil
}

func (pd *PortDaemon) getOrCreateInFlight(key string) (*inFlightRequest, bool) {
	
	pd.inFlightMutex.Lock()
	defer pd.inFlightMutex.Unlock()

	if req, exists := pd.inFlight[key]; exists {
		return req, true
	}

	req := &inFlightRequest{
		ch:   make(chan SerialResponse, 1),
		done: make(chan struct{}),
	}
	pd.inFlight[key] = req
	return req, false
}

func (pd *PortDaemon) removeInFlight(key string) {
	pd.inFlightMutex.Lock()
	delete(pd.inFlight, key)
	pd.inFlightMutex.Unlock()
}

func (pd *PortDaemon) cacheCleaner() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pd.cacheMutex.Lock()
			now := time.Now()
			initialSize := len(pd.cmdCache)
			for key, entry := range pd.cmdCache {
				if now.Sub(entry.timestamp) > entry.ttl {
					delete(pd.cmdCache, key)
				}
			}
			if len(pd.cmdCache) != initialSize {
				log.Printf("[PortDaemon] Cache cleaned: %d -> %d entries", initialSize, len(pd.cmdCache))
			}
			pd.cacheMutex.Unlock()
		case <-pd.quit:
			return
		}
	}
}

func getCacheKey(cmd string) string {

	normalized := strings.ReplaceAll(strings.TrimSpace(cmd), "\r\n", "")
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:8])
}

func getCommandTTL(cmd string) time.Duration {

	if strings.Contains(cmd, `AT+QGDCNT?`) ||
		strings.Contains(cmd, `AT+QGDNRCNT?`) ||
		strings.Contains(cmd, `AT+CSMS`) ||
		strings.Contains(cmd, `AT+CMGD`) {
		return -1
	}

	
	if strings.Contains(cmd, `AT+QENG="servingcell"`) {
		return 200 * time.Millisecond
	}

	if strings.Contains(cmd, `AT+QSIMSTAT?`) ||
		strings.Contains(cmd, `AT+QMAP=`) {
		return 30 * time.Second
	}

	if strings.Contains(cmd, `AT+QSPN`) ||
		strings.Contains(cmd, `AT+CGCONTRDP`) {
		return 90 * time.Second
	}

	return 60 * time.Second
}

func (pd *PortDaemon) Query(req SerialRequest) (SerialResponse, error) {
	
	if !pd.running {
		return SerialResponse{}, errors.New("daemon not running")
	}

	cmdStr := strings.TrimSpace(string(req.Data))
	ttl := getCommandTTL(cmdStr)

	if ttl < 0 {
		log.Printf("[PortDaemon] COMMAND NON-CACHEABLE: %s", cmdStr)
		return pd.executeQuery(req)
	}

	cacheKey := getCacheKey(cmdStr)

	pd.cacheMutex.RLock()
	entry, exists := pd.cmdCache[cacheKey]
	pd.cacheMutex.RUnlock()

	if exists && time.Since(entry.timestamp) <= entry.ttl {
		log.Printf("[PortDaemon] CACHE HIT for command: %s (age: %v, TTL: %v)",
			cmdStr, time.Since(entry.timestamp), entry.ttl)
		return SerialResponse{Data: entry.response}, nil
	}

	inFlightReq, exists := pd.getOrCreateInFlight(cacheKey)
	if exists {
		log.Printf("[PortDaemon] REQUEST COALESCING: waiting for in-flight request: %s", cmdStr)
		select {
		case resp := <-inFlightReq.ch:
			return resp, nil
		case <-time.After(req.Timeout):
			return SerialResponse{}, errors.New("coalesced request timeout")
		}
	}

	resp, err := pd.executeQuery(req)

	if err == nil && resp.Err == nil && len(resp.Data) > 0 && ttl > 0 {
		pd.cacheMutex.Lock()
		pd.cmdCache[cacheKey] = cacheEntry{
			response:  resp.Data,
			timestamp: time.Now(),
			ttl:       ttl,
		}
		pd.cacheMutex.Unlock()
		log.Printf("[PortDaemon] CACHE STORE for command: %s (TTL: %v)", cmdStr, ttl)
	}
	
	inFlightReq.ch <- resp
	close(inFlightReq.done)
	pd.removeInFlight(cacheKey)

	return resp, err
}

func (pd *PortDaemon) executeQuery(req SerialRequest) (SerialResponse, error) {

	requestID := time.Now().UnixNano()
	pd.activeRequests.Store(requestID, time.Now())
	defer pd.activeRequests.Delete(requestID)

	reply := make(chan SerialResponse, 1)

	select {
	case pd.reqChan <- msgIn{req: req, ch: reply}:
	case <-time.After(time.Second * 5):
		return SerialResponse{}, errors.New("serial daemon channel full")
	}

	select {
	case rsp := <-reply:
		return rsp, nil
	case <-time.After(req.Timeout):
		return SerialResponse{}, errors.New("serial daemon response timeout")
	}
}

func (pd *PortDaemon) run() {
	
	defer func() {
		if pd.port != nil {
			pd.port.Close()
		}
	}()

	for {
		select {
		case m := <-pd.reqChan:
			if !pd.running {
				m.ch <- SerialResponse{Err: errors.New("daemon not running")}
				continue
			}
			pd.cmdMu.Lock()
			pd.processCommand(m)
			pd.cmdMu.Unlock()
		case <-pd.quit:
			log.Println("[PortDaemon] Quit signal received. Shutting down.")
			pd.running = false
			return
		}
	}
}

func (pd *PortDaemon) processCommand(m msgIn) {
	
	startTime := time.Now()

	cmdStr := strings.TrimSpace(string(m.req.Data))
	log.Printf("[PortDaemon] sending command: %s", cmdStr)

	isLongCommand := len(m.req.Data) > 50
	isSMSCommand := strings.Contains(cmdStr, "+CMGL") ||
		strings.Contains(cmdStr, "+CSMS") ||
		strings.Contains(cmdStr, "+CNMI")

	var effectiveTimeout time.Duration = m.req.Timeout
	if isLongCommand || isSMSCommand {
		effectiveTimeout = m.req.Timeout * 2
		log.Printf("[PortDaemon] Extended timeout: %v -> %v", m.req.Timeout, effectiveTimeout)
	}

	if strings.Contains(cmdStr, "+CMGS=") {
		pd.handleSendSMSCommand(m, cmdStr, effectiveTimeout, startTime)
	} else {
		pd.handleNormalCommand(m, cmdStr, effectiveTimeout, startTime)
	}
}

func (pd *PortDaemon) handleNormalCommand(m msgIn, cmdStr string, effectiveTimeout time.Duration, startTime time.Time) {
    ctx, cancel := context.WithTimeout(context.Background(), effectiveTimeout)
    defer cancel()

    if _, err := pd.port.Write(m.req.Data); err != nil {
        log.Printf("[PortDaemon] write error: %v", err)
        m.ch <- SerialResponse{Err: err}
        return
    }

    buf := make([]byte, 4096)
    var response []byte
    lastDataTime := time.Now()
    dataTimeout := 500 * time.Millisecond
    
    consecutiveTimeouts := 0
    maxConsecutiveTimeouts := 20
	
    for {
        select {
        case <-ctx.Done():
            log.Printf("[PortDaemon] TOTAL TIMEOUT for command: %s", cmdStr)
            m.ch <- SerialResponse{Err: errors.New("serial daemon response timeout")}
            return
        default:
        }

        n, err := pd.port.Read(buf)
        
        if err != nil {
            if strings.Contains(err.Error(), "timeout") {
                consecutiveTimeouts++
                
				if consecutiveTimeouts > maxConsecutiveTimeouts {
                    log.Printf("[PortDaemon] SERIAL PORT UNRESPONSIVE for command: %s", cmdStr)
                    m.ch <- SerialResponse{Err: errors.New("serial port unresponsive")}
                    return
                }
                
				if time.Since(startTime) > effectiveTimeout {
                    log.Printf("[PortDaemon] TOTAL TIMEOUT for command: %s", cmdStr)
                    m.ch <- SerialResponse{Err: errors.New("serial daemon response timeout")}
                    return
                }
                
                if time.Since(lastDataTime) > dataTimeout && len(response) > 0 {
                    respStr := string(response)
                    if strings.Contains(respStr, "\r\nOK\r\n") || strings.Contains(respStr, "\r\nERROR\r\n") ||
                        strings.Contains(respStr, "+CME ERROR") || strings.Contains(respStr, "CONNECT") ||
                        strings.Contains(respStr, "NO CARRIER") {
                        log.Printf("[PortDaemon] RESPONSE COMPLETE for command: %s", cmdStr)
                        m.ch <- SerialResponse{Data: response}
                        return
                    }
                }
                continue
            }

            log.Printf("[PortDaemon] read error: %v", err)
            m.ch <- SerialResponse{Err: err}
            return
        }

        consecutiveTimeouts = 0

        if n > 0 {
            lastDataTime = time.Now()
            data := make([]byte, n)
            copy(data, buf[:n])
            response = append(response, data...)
            log.Printf("[ReadLoop] Read raw buffer: %s", string(data))

            respStr := string(response)
            if strings.Contains(respStr, "\r\nOK\r\n") || strings.Contains(respStr, "\r\nERROR\r\n") ||
                strings.Contains(respStr, "+CME ERROR") || strings.Contains(respStr, "\r\nCONNECT\r\n") ||
                strings.Contains(respStr, "\r\nNO CARRIER\r\n") {
                log.Printf("[PortDaemon] RESPONSE COMPLETE for command: %s", cmdStr)
                m.ch <- SerialResponse{Data: response}
                return
            }
        }
    }
}

func (pd *PortDaemon) handleSendSMSCommand(m msgIn, cmdStr string, effectiveTimeout time.Duration, startTime time.Time) {

	if _, err := pd.port.Write(m.req.Data); err != nil {
		log.Printf("[PortDaemon] write error: %v", err)
		m.ch <- SerialResponse{Err: err}
		return
	}

	buf := make([]byte, 128)
	var response []byte
	dataTimeout := 1 * time.Second
	lastDataTime := time.Now()
	
    consecutiveTimeouts := 0
    maxConsecutiveTimeouts := 30
	
	for {

		if time.Since(startTime) > effectiveTimeout {
			log.Printf("[PortDaemon] TOTAL TIMEOUT waiting for > prompt for command: %s", cmdStr)
			m.ch <- SerialResponse{Err: errors.New("serial daemon response timeout waiting for > prompt")}
			pd.port.Write([]byte("\r\nAT\r\n"))
            time.Sleep(100 * time.Millisecond)
			return
		}

		n, err := pd.port.Read(buf)
		if err != nil {
			if err.Error() == "timeout" {
				if time.Since(lastDataTime) > dataTimeout {
					consecutiveTimeouts++
					if consecutiveTimeouts > maxConsecutiveTimeouts {
						log.Printf("[PortDaemon] SERIAL PORT UNRESPONSIVE for SMS command: %s", cmdStr)
						m.ch <- SerialResponse{Err: errors.New("serial port unresponsive")}
						
						pd.port.Write([]byte("\r\nAT\r\n"))
						time.Sleep(100 * time.Millisecond)
						
						return
					}
					log.Printf("[PortDaemon] No data received for %s, continuing to wait for > prompt", dataTimeout)
					lastDataTime = time.Now()
				}
				continue
			}
			log.Printf("[PortDaemon] read error waiting for > prompt: %v", err)
			m.ch <- SerialResponse{Err: err}
			return
		} else {
            consecutiveTimeouts = 0
		}

		if n > 0 {
			lastDataTime = time.Now()
			data := make([]byte, n)
			copy(data, buf[:n])
			response = append(response, data...)
			
			log.Printf("[ReadLoop] Read raw buffer: %s", string(data))
			
			if bytes.Contains(data, []byte(">")) {
				log.Println("[PortDaemon] Received > prompt")
				m.ch <- SerialResponse{Data: response}
				return
			}
		}
	}
}

func (pd *PortDaemon) checkStuckRequests() {

	pd.activeRequests.Range(func(key, value interface{}) bool {
		startTime := value.(time.Time)
		if time.Since(startTime) > 30*time.Second {
			log.Printf("[PortDaemon] Detected stuck request ID %v, started at %v", key, startTime)
		}
		return true
	})
}

func (pd *PortDaemon) Stop() {
    if pd.running {
        pd.running = false
        close(pd.quit)
        
		if pd.port != nil {
            pd.port.SetDTR(false)
            pd.port.SetRTS(false)
            time.Sleep(100 * time.Millisecond)
            pd.port.Close()
        }
    }
}


const restartInterval = 3 * time.Second

type SerialSupervisor struct {
	portname string
	baudrate int

	mu      sync.RWMutex
	daemon  *PortDaemon
	started chan struct{}
	quit    chan struct{}
}

func NewSupervisor(portname string, baudrate int) *SerialSupervisor {

	s := &SerialSupervisor{
		portname: portname,
		baudrate: baudrate,
		started:  make(chan struct{}),
		quit:     make(chan struct{}),
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
			continue
		}

		s.mu.Lock()
		s.daemon = d
		s.mu.Unlock()

		select {
		case <-s.started:

		default:
			close(s.started)
		}

		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			
			for {
				select {
				case <-ticker.C:
					d.checkStuckRequests()
				case <-d.quit:
					return
				case <-s.quit:
					return
				}
			}
		}()

		select {
		case <-d.quit:
			log.Println("[PortDaemon] Daemon quit signal received")
		case <-s.quit:
			log.Println("[SerialSupervisor] Quit signal received. Shutting down.")
			d.Stop()
			return
		}
		
		s.mu.Lock()
		s.daemon = nil
		s.mu.Unlock()

		log.Println("[SerialSupervisor] daemon down, restart later")
		time.Sleep(restartInterval)
	}
}

func (s *SerialSupervisor) Query(req SerialRequest) (SerialResponse, error) {
	<-s.started

	s.mu.RLock()
	d := s.daemon
	s.mu.RUnlock()

	if d == nil {
		return SerialResponse{}, errors.New("no available daemon")
	}
	return d.Query(req)
}

func (s *SerialSupervisor) Stop() {
	close(s.quit)
}
