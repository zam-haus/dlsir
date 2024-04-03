package firmware

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type firmwareInfo struct {
	File      string
	Phone     string
	DevType   string
	FwType    string
	FwVersion FirmwareVersion
}

func (info firmwareInfo) IsCompatible(info2 firmwareInfo) bool {
	return info.Phone == info2.Phone && info.DevType == info2.DevType
}

func (info firmwareInfo) IsSIP() bool {
	return info.FwType == "Siemens SIP"
}

func stringFromReader(reader *bufio.Reader, file string, desc string) (string, error) {
	str, err := reader.ReadString(0)
	if err != nil {
		return "", fmt.Errorf("failed to read %v from %v: %v\n", desc, file, err)
	}
	str = strings.Trim(str, string("\x00"))
	return str, nil
}

func GetFirmwareInfo(file string) (*firmwareInfo, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("failed to open firmware file %v: %v\n", file, err)
	}
	defer f.Close()

	reader := bufio.NewReader(f)

	_, err = reader.Discard(0x20)
	if err != nil {
		return nil, err
	}

	phone, err := stringFromReader(reader, file, "phone info")
	if err != nil {
		return nil, err
	}

	// skip 0-bytes
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		if b != 0 {
			break
		}
	}

	err = reader.UnreadByte()
	if err != nil {
		return nil, err
	}

	version, err := stringFromReader(reader, file, "version info")
	if err != nil {
		return nil, err
	}

	_, err = f.Seek(-0x128, 2)
	if err != nil {
		return nil, err
	}

	reader.Reset(f)

	devType, err := stringFromReader(reader, file, "device type")
	if err != nil {
		return nil, err
	}

	_, err = reader.Discard(0x2)
	if err != nil {
		return nil, err
	}

	fwType, err := stringFromReader(reader, file, "firmware type")
	if err != nil {
		return nil, err
	}

	if fwType != "Siemens SIP" && fwType != "Siemens HFA" {
		return nil, fmt.Errorf("unknown device type'%v' - is the file a firmware image", fwType)
	}

	ver, err := ParseFirmwareVersion(version)
	if err == nil {
		return nil, fmt.Errorf("failed to parse firmware version '%v': %v", version, err)
	}

	info := firmwareInfo{File: file, Phone: phone, DevType: devType, FwType: fwType, FwVersion: *ver}
	return &info, nil
}
