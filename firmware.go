package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type firmwareVersion struct {
	Major    int
	Submajor int
	Minor    int
	Fix      int
	Hotfix   int
}

func (v firmwareVersion) Compare(o firmwareVersion) int {
	if v.Major != o.Major {
		return v.Major - o.Major
	}

	if v.Submajor != o.Submajor {
		return v.Submajor - o.Submajor
	}

	if v.Minor != o.Minor {
		return v.Minor - o.Minor
	}

	if v.Fix != o.Fix {
		return v.Fix - o.Fix
	}

	if v.Hotfix != o.Hotfix {
		return v.Hotfix - o.Hotfix
	}

	return 0
}

func (v firmwareVersion) String() string {
	if v.Submajor != 0 {
		return fmt.Sprintf("V%v.%v R%v.%v.%v", v.Major, v.Submajor, v.Minor, v.Fix, v.Hotfix)
	}
	return fmt.Sprintf("V%v R%v.%v.%v", v.Major, v.Minor, v.Fix, v.Hotfix)
}

type firmwareInfo struct {
	File      string
	Phone     string
	DevType   string
	FwType    string
	FwVersion firmwareVersion
}

func (info firmwareInfo) IsCompatible(info2 firmwareInfo) bool {
	return info.Phone == info2.Phone && info.DevType == info2.DevType
}

func (info firmwareInfo) IsSIP() bool {
	return info.FwType == "Siemens SIP"
}

func parseFirmwareVersion(version string) (*firmwareVersion, error) {
	// try to parse firmware version
	// Vx[.y] Rm.f.h
	rx := regexp.MustCompile(`V(\d+)(\.(\d+))? R(\d+)\.(\d+)\.(\d+)`)
	m := rx.FindAllStringSubmatch(version, -1)

	major, err := strconv.Atoi(m[0][1])
	if err != nil {
		return nil, fmt.Errorf("failed to parse major part (%v) of version: %v", m[0][1], err)
	}

	submajor := 0
	if m[0][3] != "" {
		submajor, err = strconv.Atoi(m[0][3])
		if err != nil {
			return nil, fmt.Errorf("failed to parse submajor part (%v) of version: %v", m[0][3], err)
		}
	}

	minor, err := strconv.Atoi(m[0][4])
	if err != nil {
		return nil, fmt.Errorf("failed to parse minor part (%v) of version: %v", m[0][4], err)
	}

	fix, err := strconv.Atoi(m[0][5])
	if err != nil {
		return nil, fmt.Errorf("failed to parse fix part (%v) of version: %v", m[0][5], err)
	}

	hotfix, err := strconv.Atoi(m[0][6])
	if err != nil {
		return nil, fmt.Errorf("failed to parse hotfix part (%v) of version: %v", m[0][6], err)
	}

	return &firmwareVersion{Major: major, Submajor: submajor, Minor: minor, Fix: fix, Hotfix: hotfix}, nil
}

func stringFromReader(reader *bufio.Reader, file string, desc string) (string, error) {
	str, err := reader.ReadString(0)
	if err != nil {
		return "", fmt.Errorf("failed to read %v from %v: %v\n", desc, file, err)
	}
	str = strings.Trim(str, string("\x00"))
	return str, nil
}

func getFirmwareInfo(file string) (*firmwareInfo, error) {
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

	ver, err := parseFirmwareVersion(version)
	if err == nil {
		return nil, fmt.Errorf("failed to parse firmware version '%v': %v", version, err)
	}

	info := firmwareInfo{File: file, Phone: phone, DevType: devType, FwType: fwType, FwVersion: *ver}
	return &info, nil
}
