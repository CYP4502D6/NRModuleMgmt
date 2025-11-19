package atserial

import (
	"fmt"
	"log"
	"sync"
	"time"
	"errors"
	"strings"
)

type NRInterfacePort struct {
	LocalBaudRate int
	LocalPort     string
	RemoteAPI     string
}

type NRInterface struct {
	IsLocal         bool
	LocalSerial     string
	LocalSerialBaud int
	RemoteSerial    string

	mu         sync.Mutex
	supervisor *SerialSupervisor
	reqID      uint32

	infoRegistry *InfoRegistry
}

func (nri *NRInterface) registerDefaultInfoProviders() {

	nri.infoRegistry.Register(&ModuleNameProvider{})
	nri.infoRegistry.Register(&ModuleCPUTempProvider{})
	nri.infoRegistry.Register(&SimStatusProvider{})
	nri.infoRegistry.Register(&SimActiveProvider{})

	nri.infoRegistry.Register(&APNProvider{})
	nri.infoRegistry.Register(&IPV4Provider{})
	nri.infoRegistry.Register(&IPV6Provider{})
	nri.infoRegistry.Register(&MCCMNCProvider{})
	nri.infoRegistry.Register(&NetworkModeProvider{})
	nri.infoRegistry.Register(&DuplexModeProvider{})
	nri.infoRegistry.Register(&CellIDProvider{})
	nri.infoRegistry.Register(&DownloadSizeProvider{})
	nri.infoRegistry.Register(&UploadSizeProvider{})

	nri.infoRegistry.Register(&LTERSRPProvider{})
	nri.infoRegistry.Register(&LTERSQProvider{})
	nri.infoRegistry.Register(&LTESINRProvider{})
	nri.infoRegistry.Register(&NRRSRPProvider{})
	nri.infoRegistry.Register(&NRRSQProvider{})
	nri.infoRegistry.Register(&NRSINRProvider{})
}

func NewNRInterface(port NRInterfacePort, isLocal bool) *NRInterface {

	nri := &NRInterface{
		IsLocal:         isLocal,
		LocalSerial:     port.LocalPort,
		LocalSerialBaud: port.LocalBaudRate,
		RemoteSerial:    port.RemoteAPI,
		infoRegistry:    NewInfoRegistry(),
	}

	if isLocal {
		log.Println("[NRInterface] create local serial daemon on", nri.LocalSerial)
		nri.supervisor = NewSupervisor(nri.LocalSerial, nri.LocalSerialBaud)
	} else {
		log.Println("[NRInterface] create remote serial via", nri.RemoteSerial)
	}

	nri.registerDefaultInfoProviders()

	return nri
}

func (nri *NRInterface) RegisterInfoProvider(provider InfoProvider) {

	nri.infoRegistry.Register(provider)
}

func (nri *NRInterface) GetInfo(key string) (interface{}, error) {

	provider, exists := nri.infoRegistry.Get(key)
	if !exists {
		return nil, errors.New("info provider not found for key: " + key)
	}

	return provider.Fetch(nri)
}

func (nri *NRInterface) GetAllInfoKeys() []string {

	return nri.infoRegistry.GetAllKeys()
}

func (nri *NRInterface) FetchAllInfo() (map[string]interface{}, error) {

	result := make(map[string]interface{})
	errors := make([]string, 0)

	keys := nri.infoRegistry.GetAllKeys()

	for _, key := range keys {
		provider, exists := nri.infoRegistry.Get(key)
		if !exists {
			errors = append(errors, fmt.Sprintf("provider not found for key: %s", key))
			continue
		}

		info, err := provider.Fetch(nri)
		if err != nil {
			errors = append(errors, fmt.Sprintf("error fetching %s: %v", key, err))
			continue
		}
		result[key] = info
	}

	if len(errors) > 0 {
		return result, fmt.Errorf("encountered %d errors: %s", len(errors), strings.Join(errors, "; "))
	}

	return result, nil
}

func (nri *NRInterface) FetchMultipleInfo(keys []string) (map[string]interface{}, error) {

	result := make(map[string]interface{})
	errors := make([]string, 0)

	for _, key := range keys {
		provider, exists := nri.infoRegistry.Get(key)
		if !exists {
			errors = append(errors, fmt.Sprintf("provider not found for key: %s", key))
			continue
		}

		info, err := provider.Fetch(nri)
		if err != nil {
			errors = append(errors, fmt.Sprintf("error fetching %s: %v", key, err))
			continue
		}

		result[key] = info
	}

	if len(errors) > 0 {
		return result, fmt.Errorf("encountered %d errors: %s", len(errors), strings.Join(errors, "; "))
	}

	return result, nil
}

func (nri *NRInterface) FetchModuleInfo() (map[string]interface{}, error) {

	keys := []string{
		"ModuleName",
		"ModuleCPUTemp",
		"SimStatus",
		"SimActive",
	}

	return nri.FetchMultipleInfo(keys)
}

func (nri *NRInterface) FetchNetworkInfo() (map[string]interface{}, error) {

	isActive, _ := nri.GetInfo("SimStatus")
	if isActive.(bool) {
		keys := []string{
			"NetworkMode",
			"DuplexMode",
			"MCCMNC",
			"APN",
			"CellID",
			"IPV4",
			"IPV6",
			"UploadSize",
			"DownloadSize",
		}

		return nri.FetchMultipleInfo(keys)
	} else {
		return nil, errors.New("network inactivity")
	}
}
func (nri *NRInterface) FetchSignalInfo(mode string) (map[string]interface{}, error) {

	if strings.Contains(mode, "NR") {
		keys := []string{
			"NR_RSRP",
			"NR_RSRQ",
			"NR_SINR",
		}
		return nri.FetchMultipleInfo(keys)
	} else if strings.Contains(mode, "LTE") {
		keys := []string{
			"LTE_RSRP",
			"LTE_RSRQ",
			"LTE_SINR",
		}
		return nri.FetchMultipleInfo(keys)
	}

	return nil, errors.New("network mode not recognized")
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
		ID:      nri.reqID,
		Data:    []byte(atcommand),
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
