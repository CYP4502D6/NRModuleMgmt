package atserial

import (
	"log"
	"time"
	"errors"
	"strconv"
	"strings"
	"encoding/hex"
	"encoding/binary"
	"unicode/utf16"
)

type NRModuleSMS struct {
	Text string
	Sender string
	Status string
	Date time.Time
	Indices int
}

func hexToUCS2(hexIn string) (string, error) {

	bytes, err := hex.DecodeString(hexIn)
	if err != nil {
		return "", errors.New("hex decode error")
	}
	
	if len(bytes) % 2 != 0 {
		return "", errors.New("invalid UTF16-BE input")
	}

	uints := make([]uint16, len(bytes) / 2)
	for i := 0; i < len(uints); i++ {
		uints[i] = binary.BigEndian.Uint16(bytes[i * 2: i * 2 + 2])
	}

	result := utf16.Decode(uints)
	return string(result), nil
}

func stringToUCS2Hex(strIn string) []byte {

	runes := []rune(strIn)
	ucs2 := utf16.Encode(runes)

	buf := make([]byte, 2 * len(ucs2))
	for i, v := range ucs2 {
		binary.BigEndian.PutUint16(buf[i * 2:], v)
	}
	return buf
}

func (nri *NRInterface) FetchSMS() ([]NRModuleSMS, error) {

	var resSMS []NRModuleSMS
	var resulterr error
	
	rawdata := nri.FetchRawData("AT+CSMS=1;+CSDH=0;+CNMI=2,1,0,0,0;+CMGF=1;+CSCA?;+CSMP=17,167,0,8;+CPMS=\"ME\",\"ME\",\"ME\";+CSCS=\"UCS2\";+CMGL=\"ALL\"\r\n")

	if strings.Contains(rawdata, "OK") {
		
		lines := strings.Split(rawdata, "\r\n")

		for index, line := range lines {
			if strings.Contains(line, "+CMGL:") {
				var smsContent string
				var smsIndices int
				var smsSender string
				var smsStatus string
				var smsDate time.Time
				if index + 1 < len(lines) {
					smsContent, _ = hexToUCS2(lines[index + 1])
				}

				ctx := strings.Split(line, ",")
				if len(ctx) >= 6 {
					smsIndices, resulterr = strconv.Atoi(strings.ReplaceAll(ctx[0], "+CMGL: ", ""))
					smsStatus = ctx[1]
					smsSender, resulterr = hexToUCS2(strings.ReplaceAll(ctx[2], "\"", ""))
					dateStr := strings.ReplaceAll(ctx[4], "\"", "") + "," + strings.ReplaceAll(ctx[5], "\"", "")
					dateStr = dateStr[:len(dateStr) - 3]
					smsDate, _ = time.Parse("06/01/02,15:04:05", dateStr)
					var sms = NRModuleSMS{
						Text: smsContent,
						Indices: smsIndices,
						Status: smsStatus,
						Sender: smsSender,
						Date: smsDate,
					}
					log.Println("[NRModuleSMS] fetch sms, sender:", smsSender, "content:", smsContent, "status:", smsStatus, "date:", dateStr)
					resSMS = append(resSMS, sms)
				} else {
					resulterr = errors.New("parse SMS failed")
				}
			}
		}
		
	} else {
		return resSMS, errors.New("fetch sms rawdata failed")
	}
	
 	return resSMS, resulterr
} 
