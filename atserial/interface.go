package atserial

import (
	"log"
	"sync"
	"time"
)

type NRInterfacePort struct {
	LocalBaudRate int
	LocalPort string
	RemoteAPI string
}

type NRInterface struct {
	IsLocal bool
	LocalSerial string
	LocalSerialBaud int
	RemoteSerial string

	mu sync.Mutex
	supervisor *SerialSupervisor
	reqID uint32
	
	ModuleInfo NRModuleInfo
	ModuleNetworkInfo NRModuleNetworkInfo
	ModuleSignalInfo NRModuleSignalInfo
}

func NewNRInterface(port NRInterfacePort, isLocal bool) *NRInterface {

	nri := &NRInterface{
		IsLocal: isLocal,
		LocalSerial: port.LocalPort,
		LocalSerialBaud: port.LocalBaudRate,
		RemoteSerial: port.RemoteAPI,
	}

	if isLocal {
		log.Println("[NRInterface] create local serial daemon on", nri.LocalSerial)
		nri.supervisor = NewSupervisor(nri.LocalSerial, nri.LocalSerialBaud)
	} else {
		log.Println("[NRInterface] create remote serial via", nri.RemoteSerial)
	}

	return nri
}

func (nri *NRInterface) FetchRawData(atcommand string, timeout time.Duration) string {

	if nri.IsLocal {
		return nri.fetchRawDataLocal(atcommand, timeout)
	}

	return nri.fetchRawDataRemote(atcommand)
}

func (nri *NRInterface) fetchRawDataLocal(atcommand string, timeout time.Duration) string {

	nri.reqID++
	req := SerialRequest{
		ID: nri.reqID,
		Data: []byte(atcommand),
		Timeout: timeout,
	}

	rsp, err := nri.supervisor.Query(req)
	if err != nil {
		log.Println("[NRInterface] serial query error:", err)
		return ""
	}
	if rsp.Err != nil {
		log.Println("[NRInterface] serial response error:", rsp.Err)
	}
	return string(rsp.Data)
}

func (nri *NRInterface) fetchRawDataRemote(atcommand string) string {
	
	log.Println("[NRInterface] http api not implemented yet")
	return ""
}

func (nri *NRInterface) Close() {
	
	if nri.supervisor != nil && nri.IsLocal {
		nri.supervisor = nil
	}
}
