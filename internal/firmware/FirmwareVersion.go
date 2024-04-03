package firmware

import (
	"fmt"
	"regexp"
	"strconv"
)

type FirmwareVersion struct {
	Major    int
	Submajor int
	Minor    int
	Fix      int
	Hotfix   int
}

func (v FirmwareVersion) Compare(o FirmwareVersion) int {
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

func (v FirmwareVersion) String() string {
	if v.Submajor != 0 {
		return fmt.Sprintf("V%v.%v R%v.%v.%v", v.Major, v.Submajor, v.Minor, v.Fix, v.Hotfix)
	}
	return fmt.Sprintf("V%v R%v.%v.%v", v.Major, v.Minor, v.Fix, v.Hotfix)
}

func ParseFirmwareVersion(version string) (*FirmwareVersion, error) {
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

	return &FirmwareVersion{Major: major, Submajor: submajor, Minor: minor, Fix: fix, Hotfix: hotfix}, nil
}
