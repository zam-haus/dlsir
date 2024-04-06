package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func itemsFromFile(c *gin.Context, confFile string) ([]item, error) {
	conf, err := os.ReadFile(confFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read file %v: %v", confFile, err)
	}

	rx := regexp.MustCompile(`\s*(?P<Key>[^\[\]= \t]+)(\[(?P<Index>\d+)\])?\s*=\s*(?P<Value>.*)\s*`)

	items := make([]item, 0)

	lines := strings.Split(string(conf), "\n")
	for lineNo, line := range lines {
		line = strings.Trim(line, " \t")
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		m := rx.FindAllStringSubmatch(line, -1)
		if m == nil {
			return nil, fmt.Errorf("line #%v '%v' has invalid format", lineNo, line)
		}

		key, indexStr, value := m[0][1], m[0][3], m[0][4]

		index := 0
		if indexStr != "" {
			index, err = strconv.Atoi(indexStr)
			if err != nil {
				return nil, fmt.Errorf("line #%v '%v' has invalid index (this should not happen)", lineNo, line)
			}
		}

		items = append(items, item{Name: key, Index: index, Value: value})
	}

	return items, nil
}

func mergeItemLists(defaults, specifics []item) []item {
	res := make([]item, len(defaults))

	// don't modify the original array, for reasons...
	copy(res, defaults)
	for _, si := range specifics {
		idx := slices.IndexFunc(res, func(i item) bool { return i.Name == si.Name && i.Index == si.Index })
		if idx != -1 {
			res[idx] = si
		} else {
			res = append(res, si)
		}
	}

	return res
}

func filterItemList(items []item, prefix string, include bool) []item {
	res := make([]item, 0)
	for _, item := range items {
		if strings.HasPrefix(item.Name, prefix) == include {
			res = append(res, item)
		}
	}

	return res
}

func getItem(items []item, name string) (item, error) {
	for _, item := range items {
		if item.Name == name {
			return item, nil
		}
	}
	return item{}, errors.New("no such item")
}

func getPhoneConfig(c *gin.Context, phone *phoneDesc, msg message) ([]item, error) {
	phoneItems, err := itemsFromFile(c, confDir+"/"+phone.Mac+".conf")
	if err != nil {
		return nil, err
	}

	defaultItems, err := itemsFromFile(c, confDir+"/phonedefault.conf")
	if err != nil {
		return nil, err
	}

	return mergeItemLists(defaultItems, phoneItems), nil
}

func getFwItemName(devType string) string {
	return "fw-" + strings.ReplaceAll(strings.ToLower(devType), " ", "")
}
