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

func parseFirmwareVersion(version string) *firmwareVersion {
	// try to parse firmware version
	// Vx[.y] Rm.f.h
	rx := regexp.MustCompile(`V(\d+)(\.(\d+))? R(\d+)\.(\d+)\.(\d+)`)
	m := rx.FindAllStringSubmatch(version, -1)

	major, err := strconv.Atoi(m[0][1])
	if err != nil {
		_log(nil, "Error: Failed to parse major part (%v) of version: %v", m[0][1], err)
		return nil
	}

	submajor := 0
	if m[0][3] != "" {
		submajor, err = strconv.Atoi(m[0][3])
		if err != nil {
			_log(nil, "Error: Failed to parse submajor part (%v) of version: %v", m[0][3], err)
			return nil
		}
	}

	minor, err := strconv.Atoi(m[0][4])
	if err != nil {
		_log(nil, "Error: Failed to parse minor part (%v) of version: %v", m[0][4], err)
		return nil
	}

	fix, err := strconv.Atoi(m[0][5])
	if err != nil {
		_log(nil, "Error: Failed to parse fix part (%v) of version: %v", m[0][5], err)
		return nil
	}

	hotfix, err := strconv.Atoi(m[0][6])
	if err != nil {
		_log(nil, "Error: Failed to parse hotfix part (%v) of version: %v", m[0][6], err)
		return nil
	}

	return &firmwareVersion{Major: major, Submajor: submajor, Minor: minor, Fix: fix, Hotfix: hotfix}
}

func stringFromReader(reader *bufio.Reader, file string, desc string) *string {
	str, err := reader.ReadString(0)
	if err != nil {
		_log(nil, "Failed to read %v from %v: %v\n", desc, file, err)
		return nil
	}
	str = strings.Trim(str, string(0))
	return &str
}

func getFirmwareInfo(file string) *firmwareInfo {
	f, err := os.Open(file)
	if err != nil {
		_log(nil, "Failed to open firmware file %v: %v\n", file, err)
		return nil
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	reader.Discard(0x20)
	phone := stringFromReader(reader, file, "phone info")

	// skip 0-bytes
	for {
		b, _ := reader.ReadByte()
		if b != 0 {
			break
		}
	}
	reader.UnreadByte()

	version := stringFromReader(reader, file, "version info")

	f.Seek(-0x128, 2)
	reader.Reset(f)
	devType := stringFromReader(reader, file, "device type")

	reader.Discard(0x2)
	fwType := stringFromReader(reader, file, "firmware type")

	if phone == nil || version == nil || devType == nil || fwType == nil {
		return nil
	}

	// _log(nil, "%20v   %v.%v.%v.%v   %10v   %v\n", *phone, major, minor, fix, hotfix, *fwType, file)

	if *fwType != "Siemens SIP" && *fwType != "Siemens HFA" {
		_log(nil, "WARNING: Unknown devType = '%v' - is the file a firmware image?\n", *fwType)
		return nil
	}

	ver := parseFirmwareVersion(*version)
	if ver == nil {
		return nil
	}

	info := firmwareInfo{File: file, Phone: *phone, DevType: *devType, FwType: *fwType, FwVersion: *ver}
	return &info
}
