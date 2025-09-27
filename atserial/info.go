package atserial

import (
	"log"
	"math"
	"errors"
	"strconv"
	"strings"
)

type NRModuleInfo struct {
	ModuleName string
	ModuleCPUTemp int

	SimStatus bool
	SimActive int
}

type NRModuleNetworkInfo struct {
	APN string
	Bands string
	CellID string
	IPV4 string
	IPV6 string
	MCCMNC int
	NetworkMode string
	DuplexMode string
	DownloadSize string
	UploadSize string
}

type NRModuleSignalInfo struct {
	LTE_RSRP int
	LTE_RSRQ int
	LTE_SINR int

	NR_RSRP int
	NR_RSRQ int
	NR_SINR int
}

func (info *NRModuleInfo) Update(nri *NRInterface) error {

	var resulterr error = nil
	rawdata := (*nri).FetchRawData("ATI;+QTEMP;+QSIMSTAT?;+QUIMSLOT?\r\n")

	if strings.Contains(rawdata, "OK") {
		infodata := strings.Split(rawdata, "\r\n")

		if len(infodata) > 2 {
			info.ModuleName = infodata[1] + infodata[2]
			for _, line := range infodata {
				
				if strings.Contains(line, "cpu0") {
					parts := strings.Split(line, ",")
					if len(parts) == 2 {
						info.ModuleCPUTemp, _ = strconv.Atoi(strings.ReplaceAll(parts[1], "\"", ""))
					}
				}

				if strings.Contains(line, "+QSIMSTAT") {
					parts := strings.Split(line, ",")
					if len(parts) == 2 {
						if parts[1] == "1" {
							info.SimStatus = true
						} else {
							info.SimStatus = false
						}
					}
				}
				
				if strings.Contains(line, "+QUIMSLOT") {
					info.SimActive, _ = strconv.Atoi(string(line[len(line)-1]))
				}
			}
		} else {
			log.Println("[NRModuleInfo] update failed, output length", len(infodata))
			resulterr = errors.New("NRModuleInfo update failed")
		}
	} else {
		log.Println("[NRModuleInfo] update failed, serial output", rawdata)
		resulterr = errors.New("NRModuleInfo update failed")
	}
	
	return resulterr
}

func (info *NRModuleNetworkInfo) Update(nri *NRInterface) error {

	var resulterr error = nil
	var non_nr_download, non_nr_upload, nr_download, nr_upload int
	
	if !(*nri).ModuleInfo.SimStatus {
		return errors.New("network not connected")
	}
	
	rawdata := (*nri).FetchRawData("AT+QSPN;+QGDCNT?;+QGDNRCNT?;+CGCONTRDP;+QENG=\"servingcell\";+QMAP=\"WWAN\"\r\n")
	
	if strings.Contains(rawdata, "OK") {
		infodata := strings.Split(rawdata, "\r\n")

		for _, line := range infodata {

			if strings.Contains(line, "+QSPN") {
				parts := strings.Split(line, ",")
				if len(parts) >= 5 {
					info.MCCMNC, _ = strconv.Atoi(strings.ReplaceAll(parts[4], "\"", ""))
				}
			}
			
			if strings.Contains(line, "+CGCONTRDP: 1") {
				parts := strings.Split(line, ",")
				if len(parts) >= 3 {
					info.APN = strings.ReplaceAll(parts[2], "\"", "")
				}
			}

			if strings.Contains(line, "IPV4") {
				parts := strings.Split(line, ",")
				info.IPV4 = strings.ReplaceAll(parts[4], "\"", "")
			}

			if strings.Contains(line, "IPV6") {
				parts := strings.Split(line, ",")
				info.IPV6 = strings.ReplaceAll(parts[4], "\"", "")
			}

			if strings.Contains(line, "servingcell") {
				parts := strings.Split(line, ",")
				if len(parts) >= 7 {
					info.NetworkMode = strings.ReplaceAll(parts[2], "\"", "")
					info.DuplexMode = strings.ReplaceAll(parts[3], "\"", "")
					info.CellID = strings.ReplaceAll(parts[6], "\"", "")
				}
			}

			if strings.Contains(line, "+QGDNRCNT") {
				parts := strings.Split(line, ",")
				if len(parts) >= 2 {
					nr_download, _ = strconv.Atoi(strings.ReplaceAll(parts[0], "+QGDNRCNT: ", ""))
					nr_upload, _ = strconv.Atoi(parts[1])
				}
			}
			
			if strings.Contains(line, "+QGDCNT") {
				parts := strings.Split(line, ",")
				if len(parts) >= 2 {
					nr_download, _ = strconv.Atoi(strings.ReplaceAll(parts[0], "+QGDCNT: ", ""))
					nr_upload, _ = strconv.Atoi(parts[1])
				}
			}
			
			info.DownloadSize = bytesToSize(float64(nr_download + non_nr_download))
			info.UploadSize = bytesToSize(float64(nr_upload + non_nr_upload))
			
		}
	} else {
		log.Println("[NRModuleNetworkInfo] update failed, rawdata:", rawdata)
		resulterr = errors.New("NRModuleNetworkInfo update failed")
	}
	
	return resulterr
}

func (info *NRModuleSignalInfo) Update(nri *NRInterface) error {

	var resulterr error = nil

	if !(*nri).ModuleInfo.SimStatus {
		return errors.New("network not connected")
	}

	if strings.Contains((*nri).ModuleNetworkInfo.NetworkMode, "5G") {
		rawdata := (*nri).FetchRawData("AT+QENG=\"servingcell\"\r\n")

		if strings.Contains(rawdata, "OK") {
			infodata := strings.Split(rawdata, "\r\n")

			for _, line := range infodata {
				if strings.Contains(line, "servingcell") {
					parts := strings.Split(line, ",")
					if len(parts) >= 15 {
						info.NR_RSRP, _ = strconv.Atoi(strings.ReplaceAll(parts[12], "\"", ""))
						info.NR_RSRQ, _ = strconv.Atoi(strings.ReplaceAll(parts[13], "\"", ""))
						info.NR_SINR, _ = strconv.Atoi(strings.ReplaceAll(parts[14], "\"", ""))
					}
				}
			}
		}
		
	} else if strings.Contains((*nri).ModuleNetworkInfo.NetworkMode, "LTE") {
		rawdata := (*nri).FetchRawData("AT+QENG=\"servingcell\"\r\n")

		if strings.Contains(rawdata, "OK") {
			infodata := strings.Split(rawdata, "\r\n")

			for _, line := range infodata {
				if strings.Contains(line, "servingcell") {
					parts := strings.Split(line, ",")
					if len(parts) >= 15 {
						info.LTE_RSRP, _ = strconv.Atoi(strings.ReplaceAll(parts[12], "\"", ""))
						info.LTE_RSRQ, _ = strconv.Atoi(strings.ReplaceAll(parts[13], "\"", ""))
						info.LTE_SINR, _ = strconv.Atoi(strings.ReplaceAll(parts[14], "\"", ""))
					}
				}
			}
		}
	} else {
		return errors.New("no signal info")
	}
	return resulterr
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
