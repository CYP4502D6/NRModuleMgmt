package atserial

import (
	"fmt"
	"math"
	"time"
	"errors"
	"strconv"
	"strings"
)

type InfoProvider interface {
	GetKey() string
	Fetch(nri *NRInterface) (interface{}, error)
}

type InfoRegistry struct {
	providers map[string]InfoProvider
}

func NewInfoRegistry() *InfoRegistry {
	return &InfoRegistry{
		providers: make(map[string]InfoProvider),
	}
}

func (r *InfoRegistry) Register(provider InfoProvider) {
	r.providers[provider.GetKey()] = provider
}

func (r *InfoRegistry) Get(key string) (InfoProvider, bool) {
	provider, ok := r.providers[key]
	return provider, ok
}

func (r *InfoRegistry) GetAllKeys() []string {
	keys := make([]string, 0, len(r.providers))

	for k := range r.providers {
		keys = append(keys, k)
	}

	return keys
}

func bytesToSize(bytes float64) string {
	
	sizes := [5]string{"Bytes", "KiB", "MiB", "GiB", "TiB"}

	if bytes == 0 {
		return "0 Byte"
	}
	tmp := int(math.Floor(math.Log(bytes) / math.Log(1024)))
	result := math.Round(bytes / math.Pow(1024, float64(tmp)))

	return strconv.FormatFloat(result, 'f', 4, 64) + sizes[tmp]
}

type ModuleNameProvider struct{}
func (p *ModuleNameProvider) GetKey() string { return "ModuleName" }
func (p *ModuleNameProvider) Fetch(nri *NRInterface) (interface{}, error) {
	
	rawdata := nri.FetchRawData("ATI\r\n", time.Second)
	
	if !strings.Contains(rawdata, "OK") {
		return "", errors.New("fetch module name failed")
	}
	infodata := strings.Split(rawdata, "\r\n")
	if len(infodata) > 2 {
		return infodata[1] + infodata[2], nil
	}
	
	return "", errors.New("invalid response length")
}

type ModuleCPUTempProvider struct{}
func (p *ModuleCPUTempProvider) GetKey() string { return "ModuleCPUTemp" }
func (p *ModuleCPUTempProvider) Fetch(nri *NRInterface) (interface{}, error) {
	
	rawdata := nri.FetchRawData("AT+QTEMP\r\n", time.Second)
	
	if !strings.Contains(rawdata, "OK") {
		return 0, errors.New("fetch CPU temp failed")
	}
	infodata := strings.Split(rawdata, "\r\n")
	for _, line := range infodata {
		if strings.Contains(line, "cpu0") {
			parts := strings.Split(line, ",")
			if len(parts) == 2 {
				return strconv.Atoi(strings.ReplaceAll(parts[1], "\"", ""))
			}
		}
	}
	
	return 0, errors.New("CPU temp not found")
}

type SimStatusProvider struct{}
func (p *SimStatusProvider) GetKey() string { return "SimStatus" }
func (p *SimStatusProvider) Fetch(nri *NRInterface) (interface{}, error) {
	
	rawdata := nri.FetchRawData("AT+QSIMSTAT?\r\n", time.Second)
	
	if !strings.Contains(rawdata, "OK") {
		return false, errors.New("fetch SIM status failed")
	}
	infodata := strings.Split(rawdata, "\r\n")
	for _, line := range infodata {
		if strings.Contains(line, "+QSIMSTAT") {
			parts := strings.Split(line, ",")
			if len(parts) == 2 {
				return parts[1] == "1", nil
			}
		}
	}
	
	return false, errors.New("SIM status not found")
}

type SimActiveProvider struct{}
func (p *SimActiveProvider) GetKey() string { return "SimActive" }
func (p *SimActiveProvider) Fetch(nri *NRInterface) (interface{}, error) {
	
	rawdata := nri.FetchRawData("AT+QUIMSLOT?\r\n", time.Second)
	
	if !strings.Contains(rawdata, "OK") {
		return 0, errors.New("fetch SIM active slot failed")
	}
	infodata := strings.Split(rawdata, "\r\n")
	for _, line := range infodata {
		if strings.Contains(line, "+QUIMSLOT:") {
			fmt.Println(line)
			line = strings.ReplaceAll(line, "\r", "")
			if len(line) > 0 {
				return strconv.Atoi(string(line[len(line)-1]))
			}
		}
	}
	
	return 0, errors.New("SIM active slot not found")
}

type APNProvider struct{}
func (p *APNProvider) GetKey() string { return "APN" }
func (p *APNProvider) Fetch(nri *NRInterface) (interface{}, error) {
	
	rawdata := nri.FetchRawData("AT+CGCONTRDP\r\n", time.Second)
	
	if !strings.Contains(rawdata, "OK") {
		return "", errors.New("fetch APN failed")
	}
	infodata := strings.Split(rawdata, "\r\n")
	for _, line := range infodata {
		if strings.Contains(line, "+CGCONTRDP: 1") {
			parts := strings.Split(line, ",")
			if len(parts) >= 3 {
				return strings.ReplaceAll(parts[2], "\"", ""), nil
			}
		}
	}
	
	return "", errors.New("APN not found")
}

type IPV4Provider struct{}
func (p *IPV4Provider) GetKey() string { return "IPV4" }
func (p *IPV4Provider) Fetch(nri *NRInterface) (interface{}, error) {
	
	rawdata := nri.FetchRawData("AT+QMAP=\"WWAN\"\r\n", time.Second)
	
	if !strings.Contains(rawdata, "OK") {
		return "", errors.New("fetch IPv4 failed")
	}
	infodata := strings.Split(rawdata, "\r\n")
	for _, line := range infodata {
		if strings.Contains(line, "IPV4") {
			parts := strings.Split(line, ",")
			if len(parts) >= 5 {
				return strings.ReplaceAll(parts[4], "\"", ""), nil
			}
		}
	}
	
	return "", errors.New("IPv4 not found")
}

type IPV6Provider struct{}
func (p *IPV6Provider) GetKey() string { return "IPV6" }
func (p *IPV6Provider) Fetch(nri *NRInterface) (interface{}, error) {
	
	rawdata := nri.FetchRawData("AT+QMAP=\"WWAN\"\r\n", time.Second)
	
	if !strings.Contains(rawdata, "OK") {
		return "", errors.New("fetch IPv6 failed")
	}
	infodata := strings.Split(rawdata, "\r\n")
	for _, line := range infodata {
		if strings.Contains(line, "IPV6") {
			parts := strings.Split(line, ",")
			if len(parts) >= 5 {
				return strings.ReplaceAll(parts[4], "\"", ""), nil
			}
		}
	}
	
	return "", errors.New("IPv6 not found")
}

type MCCMNCProvider struct{}
func (p *MCCMNCProvider) GetKey() string { return "MCCMNC" }
func (p *MCCMNCProvider) Fetch(nri *NRInterface) (interface{}, error) {
	
	rawdata := nri.FetchRawData("AT+QSPN\r\n", time.Second)
	
	if !strings.Contains(rawdata, "OK") {
		return 0, errors.New("fetch MCCMNC failed")
	}
	infodata := strings.Split(rawdata, "\r\n")
	for _, line := range infodata {
		if strings.Contains(line, "+QSPN") {
			parts := strings.Split(line, ",")
			if len(parts) >= 5 {
				return strconv.Atoi(strings.ReplaceAll(parts[4], "\"", ""))
			}
		}
	}
	
	return 0, errors.New("MCCMNC not found")
}

type NetworkModeProvider struct{}
func (p *NetworkModeProvider) GetKey() string { return "NetworkMode" }
func (p *NetworkModeProvider) Fetch(nri *NRInterface) (interface{}, error) {
	
	rawdata := nri.FetchRawData("AT+QENG=\"servingcell\"\r\n", time.Second)
	
	if !strings.Contains(rawdata, "OK") {
		return "", errors.New("fetch network mode failed")
	}
	infodata := strings.Split(rawdata, "\r\n")
	for _, line := range infodata {
		if strings.Contains(line, "servingcell") {
			parts := strings.Split(line, ",")
			if len(parts) >= 4 {
				return strings.ReplaceAll(parts[2], "\"", ""), nil
			}
		}
	}
	
	return "", errors.New("Network mode not found")
}

type DuplexModeProvider struct{}
func (p *DuplexModeProvider) GetKey() string { return "DuplexMode" }
func (p *DuplexModeProvider) Fetch(nri *NRInterface) (interface{}, error) {
	
	rawdata := nri.FetchRawData("AT+QENG=\"servingcell\"\r\n", time.Second)
	
	if !strings.Contains(rawdata, "OK") {
		return "", errors.New("fetch duplex mode failed")
	}
	infodata := strings.Split(rawdata, "\r\n")
	for _, line := range infodata {
		if strings.Contains(line, "servingcell") {
			parts := strings.Split(line, ",")
			if len(parts) >= 4 {
				return strings.ReplaceAll(parts[3], "\"", ""), nil
			}
		}
	}
	
	return "", errors.New("Duplex mode not found")
}

type CellIDProvider struct{}
func (p *CellIDProvider) GetKey() string { return "CellID" }
func (p *CellIDProvider) Fetch(nri *NRInterface) (interface{}, error) {
	
	rawdata := nri.FetchRawData("AT+QENG=\"servingcell\"\r\n", time.Second)
	
	if !strings.Contains(rawdata, "OK") {
		return "", errors.New("fetch cell ID failed")
	}
	infodata := strings.Split(rawdata, "\r\n")
	for _, line := range infodata {
		if strings.Contains(line, "servingcell") {
			parts := strings.Split(line, ",")
			if len(parts) >= 7 {
				return strings.ReplaceAll(parts[6], "\"", ""), nil
			}
		}
	}
	
	return "", errors.New("Cell ID not found")
}

type DownloadSizeProvider struct{}
func (p *DownloadSizeProvider) GetKey() string { return "DownloadSize" }
func (p *DownloadSizeProvider) Fetch(nri *NRInterface) (interface{}, error) {
	
	var totalDownload int
	
	rawdata := nri.FetchRawData("AT+QGDCNT?\r\n", time.Second)
	
	if strings.Contains(rawdata, "OK") {
		infodata := strings.Split(rawdata, "\r\n")
		for _, line := range infodata {
			if strings.Contains(line, "+QGDCNT") {
				parts := strings.Split(line, ",")
				if len(parts) >= 2 {
					download, _ := strconv.Atoi(strings.ReplaceAll(parts[0], "+QGDCNT: ", ""))
					totalDownload += download
				}
			}
		}
	}
	
	rawdata = nri.FetchRawData("AT+QGDNRCNT?\r\n", time.Second)
	
	if strings.Contains(rawdata, "OK") {
		infodata := strings.Split(rawdata, "\r\n")
		for _, line := range infodata {
			if strings.Contains(line, "+QGDNRCNT") {
				parts := strings.Split(line, ",")
				if len(parts) >= 2 {
					download, _ := strconv.Atoi(strings.ReplaceAll(parts[0], "+QGDNRCNT: ", ""))
					totalDownload += download
				}
			}
		}
	}
	
	return bytesToSize(float64(totalDownload)), nil
}

type UploadSizeProvider struct{}
func (p *UploadSizeProvider) GetKey() string { return "UploadSize" }
func (p *UploadSizeProvider) Fetch(nri *NRInterface) (interface{}, error) {
	var totalUpload int
	
	rawdata := nri.FetchRawData("AT+QGDCNT?\r\n", time.Second)
	
	if strings.Contains(rawdata, "OK") {
		infodata := strings.Split(rawdata, "\r\n")
		for _, line := range infodata {
			if strings.Contains(line, "+QGDCNT") {
				parts := strings.Split(line, ",")
				if len(parts) >= 2 {
					upload, _ := strconv.Atoi(parts[1])
					totalUpload += upload
				}
			}
		}
	}
	
	rawdata = nri.FetchRawData("AT+QGDNRCNT?\r\n", time.Second)
	
	if strings.Contains(rawdata, "OK") {
		infodata := strings.Split(rawdata, "\r\n")
		for _, line := range infodata {
			if strings.Contains(line, "+QGDNRCNT") {
				parts := strings.Split(line, ",")
				if len(parts) >= 2 {
					upload, _ := strconv.Atoi(parts[1])
					totalUpload += upload
				}
			}
		}
	}
	
	return bytesToSize(float64(totalUpload)), nil
}

type LTERSRPProvider struct{}
func (p *LTERSRPProvider) GetKey() string { return "LTE_RSRP" }
func (p *LTERSRPProvider) Fetch(nri *NRInterface) (interface{}, error) {
	
	networkMode, err := nri.GetInfo("NetworkMode")
	
	if err != nil {
		return 0, err
	}
	
	mode, ok := networkMode.(string)
	if !ok || !strings.Contains(mode, "LTE") {
		return 0, errors.New("not in LTE mode")
	}
	
	rawdata := nri.FetchRawData("AT+QENG=\"servingcell\"\r\n", time.Second)
	
	if !strings.Contains(rawdata, "OK") {
		return 0, errors.New("fetch LTE RSRP failed")
	}
	infodata := strings.Split(rawdata, "\r\n")
	for _, line := range infodata {
		if strings.Contains(line, "servingcell") {
			parts := strings.Split(line, ",")
			if len(parts) >= 15 {
				return strconv.Atoi(strings.ReplaceAll(parts[12], "\"", ""))
			}
		}
	}
	return 0, errors.New("LTE RSRP not found")
}

type LTERSQProvider struct{}
func (p *LTERSQProvider) GetKey() string { return "LTE_RSRQ" }
func (p *LTERSQProvider) Fetch(nri *NRInterface) (interface{}, error) {
	
	networkMode, err := nri.GetInfo("NetworkMode")
	
	if err != nil {
		return 0, err
	}
	
	mode, ok := networkMode.(string)
	if !ok || !strings.Contains(mode, "LTE") {
		return 0, errors.New("not in LTE mode")
	}
	
	rawdata := nri.FetchRawData("AT+QENG=\"servingcell\"\r\n", time.Second)
	
	if !strings.Contains(rawdata, "OK") {
		return 0, errors.New("fetch LTE RSRQ failed")
	}
	infodata := strings.Split(rawdata, "\r\n")
	for _, line := range infodata {
		if strings.Contains(line, "servingcell") {
			parts := strings.Split(line, ",")
			if len(parts) >= 15 {
				return strconv.Atoi(strings.ReplaceAll(parts[13], "\"", ""))
			}
		}
	}
	return 0, errors.New("LTE RSRQ not found")
}

type LTESINRProvider struct{}
func (p *LTESINRProvider) GetKey() string { return "LTE_SINR" }
func (p *LTESINRProvider) Fetch(nri *NRInterface) (interface{}, error) {
	
	networkMode, err := nri.GetInfo("NetworkMode")
	
	if err != nil {
		return 0, err
	}
	
	mode, ok := networkMode.(string)

	if !ok || !strings.Contains(mode, "LTE") {
		return 0, errors.New("not in LTE mode")
	}
	
	rawdata := nri.FetchRawData("AT+QENG=\"servingcell\"\r\n", time.Second)
	
	if !strings.Contains(rawdata, "OK") {
		return 0, errors.New("fetch LTE SINR failed")
	}
	infodata := strings.Split(rawdata, "\r\n")
	for _, line := range infodata {
		if strings.Contains(line, "servingcell") {
			parts := strings.Split(line, ",")
			if len(parts) >= 15 {
				return strconv.Atoi(strings.ReplaceAll(parts[14], "\"", ""))
			}
		}
	}
	
	return 0, errors.New("LTE SINR not found")
}

type NRRSRPProvider struct{}
func (p *NRRSRPProvider) GetKey() string { return "NR_RSRP" }
func (p *NRRSRPProvider) Fetch(nri *NRInterface) (interface{}, error) {
	
	networkMode, err := nri.GetInfo("NetworkMode")
	
	if err != nil {
		return 0, err
	}
	
	mode, ok := networkMode.(string)
	if !ok || !strings.Contains(mode, "5G") {
		return 0, errors.New("not in 5G mode")
	}
	
	rawdata := nri.FetchRawData("AT+QENG=\"servingcell\"\r\n", time.Second)
	
	if !strings.Contains(rawdata, "OK") {
		return 0, errors.New("fetch NR RSRP failed")
	}
	infodata := strings.Split(rawdata, "\r\n")
	for _, line := range infodata {
		if strings.Contains(line, "servingcell") {
			parts := strings.Split(line, ",")
			if len(parts) >= 15 {
				return strconv.Atoi(strings.ReplaceAll(parts[12], "\"", ""))
			}
		}
	}
	return 0, errors.New("NR RSRP not found")
}

type NRRSQProvider struct{}
func (p *NRRSQProvider) GetKey() string { return "NR_RSRQ" }
func (p *NRRSQProvider) Fetch(nri *NRInterface) (interface{}, error) {
	
	networkMode, err := nri.GetInfo("NetworkMode")
	
	if err != nil {
		return 0, err
	}
	
	mode, ok := networkMode.(string)
	if !ok || !strings.Contains(mode, "5G") {
		return 0, errors.New("not in 5G mode")
	}
	
	rawdata := nri.FetchRawData("AT+QENG=\"servingcell\"\r\n", time.Second)
	
	if !strings.Contains(rawdata, "OK") {
		return 0, errors.New("fetch NR RSRQ failed")
	}
	infodata := strings.Split(rawdata, "\r\n")
	for _, line := range infodata {
		if strings.Contains(line, "servingcell") {
			parts := strings.Split(line, ",")
			if len(parts) >= 15 {
				return strconv.Atoi(strings.ReplaceAll(parts[13], "\"", ""))
			}
		}
	}
	return 0, errors.New("NR RSRQ not found")
}

type NRSINRProvider struct{}
func (p *NRSINRProvider) GetKey() string { return "NR_SINR" }
func (p *NRSINRProvider) Fetch(nri *NRInterface) (interface{}, error) {
	
	networkMode, err := nri.GetInfo("NetworkMode")
	
	if err != nil {
		return 0, err
	}
	
	mode, ok := networkMode.(string)
	if !ok || !strings.Contains(mode, "5G") {
		return 0, errors.New("not in 5G mode")
	}
	
	rawdata := nri.FetchRawData("AT+QENG=\"servingcell\"\r\n", time.Second)
	
	if !strings.Contains(rawdata, "OK") {
		return 0, errors.New("fetch NR SINR failed")
	}
	infodata := strings.Split(rawdata, "\r\n")
	for _, line := range infodata {
		if strings.Contains(line, "servingcell") {
			parts := strings.Split(line, ",")
			if len(parts) >= 15 {
				return strconv.Atoi(strings.ReplaceAll(parts[14], "\"", ""))
			}
		}
	}
	return 0, errors.New("NR SINR not found")
}
