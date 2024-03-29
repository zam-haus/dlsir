package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	// "github.com/davecgh/go-spew/spew"
)

const confDir = "./conf/"
const confSrv = confDir + "/dlsir.conf"
const confDumpDir = "./conf_dump/"

type phoneNextProvStep int

const (
	Initial          = iota // No contact yet
	WaitForSolicited        // ContactMe was sent; now wait for request from phone
	SendConfig              // Send system configuration
	SendFiles               // Send files (excluding software)
	SendSoftware            // Send system software (i.e., firmware update)
	WaitForUpdate           // Software was sent; now wait for phone response
	RequestConfig           // Request the phone's current configuration
)

func (state phoneNextProvStep) String() string {
	switch state {
	case Initial:
		return "Initial"
	case WaitForSolicited:
		return "WaitForSolicited"
	case SendConfig:
		return "SendConfig"
	case SendFiles:
		return "SendFiles"
	case SendSoftware:
		return "SendSoftware"
	case WaitForUpdate:
		return "WaitForUpdate"
	case RequestConfig:
		return "RequestConfig"

	default:
		return "INVALID"
	}
}

type phoneDesc struct {
	Mac           string
	IP            string
	Number        string
	NextStep      phoneNextProvStep
	PendingFiles  []string
	RqBegin       time.Time
	DevType       string
	FwVersion     firmwareVersion
	FwNeedsUpdate bool
}

type item struct {
	Name   string `xml:"name,attr"`
	Index  int    `xml:"index,attr,omitempty"`
	Status string `xml:"status,attr,omitempty"`
	Value  string `xml:",chardata"`
}

type reason struct {
	Action string `xml:"action,attr,omitempty"`
	Status string `xml:"status,attr,omitempty"`
	Value  string `xml:",chardata"`
}

type message struct {
	Action   string `xml:"Action,omitempty"`
	Reason   reason `xml:"ReasonForContact,-"`
	Nonce    string `xml:"nonce,attr"`
	MaxItems int    `xml:"maxItems,attr,omitempty"`
	Fragment string `xml:"fragment,attr,omitempty"`
	Items    []item `xml:"ItemList>Item"`
}

type loginServiceData struct {
	Message message `xml:"Message"`
}

type dlsMessage struct {
	XMLName struct{} `xml:"DLSMessage"`
	Message message  `xml:"Message"`
}

var phoneState map[string]*phoneDesc

func formatItemList(items []item) string {
	var sb strings.Builder

	for _, item := range items {
		sb.WriteString(item.Name)

		if item.Index != 0 {
			sb.WriteString("[")
			sb.WriteString(strconv.Itoa(item.Index))
			sb.WriteString("]")
		}

		sb.WriteString(" = ")
		sb.WriteString(item.Value)

		if item.Status != "" {
			sb.WriteString(" [")
			sb.WriteString(item.Status)
			sb.WriteString("]")
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

func sendConfig(c *gin.Context, phone *phoneDesc, msg message) (string, []item) {
	items := filterItemList(getPhoneConfig(c, phone, msg), "file-", false)
	return "WriteItems", items
}

func sendFiles(c *gin.Context, phone *phoneDesc, msg message) (string, []item) {
	items := filterItemList(getPhoneConfig(c, phone, msg), "file-", true)

	localHost := c.Request.Host

	for idx := range items {
		if items[idx].Name == "file-name" {
			items[idx].Name = "file-https-base-url"
			items[idx].Value = fmt.Sprintf("https://%v/file/%v", localHost, items[idx].Value)
		}
	}

	return "FileDeployment", items
}

func sendSoftware(c *gin.Context, phone *phoneDesc, msg message) (string, []item) {
	items := make([]item, 0)

	localHost := c.Request.Host
	config := itemsFromFile(nil, confSrv)
	fwConfigName := getFwItemName(phone.DevType)
	fwFile, err := getItem(config, fwConfigName)
	if err != nil {
		_log(c, "Failed to read firmware file from configuration; missing entry fw-openstage40")
		_log(c, "This is strange; we should not have ended up here!")
		return "", []item{}
	}

	fw, err := getFirmwareInfo("files/" + fwFile.Value)
	if err != nil {
		_log(c, "Failed to read firmware version from file; maybe not a proper firmware file?")
		_log(c, "Error: %v", err.Error())
		return "", []item{}
	}

	_log(c, "Issuing software update for phone %v / %v", phone.Number, phone.IP)
	_log(c, " - old version: %v", phone.FwVersion)
	_log(c, " - new version: %v", fw.FwVersion)

	items = append(items, item{Name: "file-https-base-url", Index: 0, Value: fmt.Sprintf("https://%v/file/%v", localHost, fwFile.Value)})
	items = append(items, item{Name: "file-priority", Index: 0, Value: "immediate"})
	items = append(items, item{Name: "file-sw-type", Index: 0, Value: fw.FwType})
	items = append(items, item{Name: "file-sw-version", Index: 0, Value: fw.FwVersion.String()})
	items = append(items, item{Name: "file-type", Index: 0, Value: "APP"})

	_log(c, "Sending: %v", formatItemList(items))

	return "SoftwareDeployment", items
}

func readAllItems(c *gin.Context, phone *phoneDesc, msg message) (string, []item) {
	return "ReadAllItems", []item{}
}

func checkReply(c *gin.Context, phone *phoneDesc, msg message) bool {
	_log(c, "Action %v was %v", msg.Reason.Action, msg.Reason.Status)
	//printItemList(c, msg.Items);

	if msg.Reason.Action == "ReadAllItems" && msg.Reason.Status == "accepted" {
		file := fmt.Sprintf("%v/%v.conf", confDumpDir, phone.Number)
		content := formatItemList(msg.Items)

		os.WriteFile(file, []byte(content), 0666)
	}

	return msg.Reason.Status == "accepted"
}

func findItem(items []item, name string, index int) *item {
	for _, item := range items {
		if item.Name == name && item.Index == index {
			return &item
		}
	}
	return nil
}

func checkStatus(c *gin.Context, phone *phoneDesc, msg message) bool {
	for _, item := range msg.Items {
		if item.Name == "file-deployment-name" {
			statusItem := findItem(msg.Items, "file-deployment-status", item.Index)

			_log(c, "  - File '%v' upload %v\n", item.Value, statusItem.Value)
		}
	}

	// _log(c, "WARNING: Status response does not contain required items!\n");
	// spew.Dump(msg)

	return true
}

func getFile(c *gin.Context) {
	file := c.Params.ByName("file")

	_log(c, "Got GET request for file %v\n", file)

	c.FileAttachment("./files/"+file, file)
}

func itemByName(items []item, name string) *string {
	idx := slices.IndexFunc(items, func(i item) bool { return i.Name == name })
	if idx == -1 {
		return nil
	}

	return &items[idx].Value
}

func postLoginService(c *gin.Context) {
	var data loginServiceData
	err := c.BindXML(&data)
	if err != nil {
		_log(c, "BindXML failed: %v\n", err)
		c.Status(http.StatusBadRequest)
		return
	}

	msg := data.Message

	// try to find phone number in response
	phoneIP := c.RemoteIP()
	if _, ok := phoneState[phoneIP]; !ok {
		phoneNoPtr := itemByName(msg.Items, "e164")
		phoneNo := "?"
		if phoneNoPtr != nil {
			phoneNo = *phoneNoPtr
		}

		phoneMac := itemByName(msg.Items, "mac-addr")
		devType := itemByName(msg.Items, "device-type")
		fwType := itemByName(msg.Items, "software-type")
		fwVersion := itemByName(msg.Items, "software-version")

		if phoneMac == nil || devType == nil || fwType == nil || fwVersion == nil {
			_log(c, "Initial contact missing required information")
			_log(c, " - mac-addr: %v", *phoneMac)
			_log(c, " - device-type: %v", *devType)
			_log(c, " - software-type: %v", *fwType)
			_log(c, " - software-version: %v", *fwVersion)

			c.Status(http.StatusBadRequest)
			return
		}

		ver := parseFirmwareVersion(*fwVersion)
		if ver == nil {
			c.Status(http.StatusBadRequest)
			return
		}

		config := itemsFromFile(nil, confSrv)
		fwConfigName := getFwItemName(*devType)
		fwFile, err := getItem(config, fwConfigName)
		needsUpdate := false
		if err == nil {
			myVersion, err := getFirmwareInfo("files/" + fwFile.Value)
			if err != nil {
				_log(c, "Failed to read firmware version from file; maybe not a proper firmware file?")
				_log(c, "Error: %v", err.Error())
			} else {
				needsUpdate = ver.Compare(myVersion.FwVersion) < 0
				if needsUpdate {
					_log(c, "Phone is running old firmware, is: %v, should be: %v", *ver, myVersion.FwVersion)
				} else {
					_log(c, "Phone is running most recent firmware %v", *ver)
				}
			}
		} else {
			_log(c, "I don't have a firmware for %v (configure as %v)", *devType, fwConfigName)
		}

		phoneState[phoneIP] = &phoneDesc{Mac: *phoneMac, IP: phoneIP, Number: phoneNo, NextStep: Initial, RqBegin: time.Now(), DevType: *devType, FwVersion: *ver, FwNeedsUpdate: needsUpdate}
	}

	phone := phoneState[phoneIP]
	_log(c, "Request from phone '%v' with reason '%v'\n", phone.Number, msg.Reason.Value)
	_log(c, " - Nonce: %v\n", msg.Nonce)
	_log(c, " - local/remote IP: %v - %v\n", c.Request.Host, c.Request.RemoteAddr)
	_log(c, " - NextStep: %v\n", phone.NextStep)

	var action string
	var responseItems []item

	if msg.Reason.Value == "start-up" && phone.NextStep == WaitForUpdate {
		// we issued a software update and the phone rebooted
		// -> software update was likely successful
		_log(c, "Yay - phone came back after a software update; requesting current configuration")

		action, responseItems = readAllItems(c, phone, msg)
		phone.NextStep = RequestConfig
	} else if msg.Reason.Value == "start-up" || msg.Reason.Value == "solicited" {
		// we send the full phone configuration both on startup and explicit request
		action, responseItems = sendConfig(c, phone, msg)
		phone.NextStep = SendFiles
	} else if msg.Reason.Value == "reply-to" {
		// this is a reply to a previous request - check the reply and continue with the next request
		wasAccepted := checkReply(c, phone, msg)

		if wasAccepted {
			if phone.NextStep == SendFiles {
				_log(c, "Configuration options sent successfully, continuing with files\n")
				action, responseItems = sendFiles(c, phone, msg)
				if phone.FwNeedsUpdate {
					phone.NextStep = SendSoftware
				} else {
					phone.NextStep = RequestConfig
				}

			} else if phone.NextStep == RequestConfig {
				_log(c, "Configuration finished successfully - current dump in %v\n", confDumpDir)
				// we're done configuring the phone; wipe phone state and wait for new requests
				delete(phoneState, phoneIP)
			}
		} else {
			_log(c, "WARNING: Phone didn't accept previous request; aborting...")
		}
	} else if msg.Reason.Value == "status" {
		// "status" is only sent after file operations
		wasAccepted := checkStatus(c, phone, msg)
		if !wasAccepted {
			_log(c, "WARNING: Phone didn't accept previous request")
		}

		if phone.NextStep == SendSoftware {
			action, responseItems = sendSoftware(c, phone, msg)
			phone.NextStep = WaitForUpdate
		} else if phone.NextStep == RequestConfig {
			action, responseItems = readAllItems(c, phone, msg)
			// phone.NextStep = RequestConfig
		} else {
			_log(c, "Error: Got unexpected NextStep = %v", phone.NextStep)
		}
	} else if msg.Reason.Value == "local-changes" {
		// we currently just ignore local changes on the phone

		itemString := formatItemList(msg.Items)

		_log(c, "Ignoring reason local-changes - we just don't care yet!")
		_log(c, "Phone sent:\n%v", itemString)
	} else {
		_log(c, "WARNING: Request reason %v is unknown/not implemented yet\n", msg.Reason.Value)
		c.Status(http.StatusNoContent)
		return
	}

	if responseItems != nil {
		//_log(c, "Sending items:\n");
		//printItemList(c, responseItems);

		// we always send a response if the handling function above provided items to send to the phone
		response := dlsMessage{Message: message{Action: action, Nonce: msg.Nonce, Items: responseItems}}

		c.XML(http.StatusOK, response)
	}
}

type connDialer struct {
	c net.Conn
}

func (cd connDialer) Dial(network, addr string) (net.Conn, error) {
	return cd.c, nil
}

func sendContactMe(listenPort, host string) {
	conn, err := net.Dial("tcp", net.JoinHostPort(host, "8085"))
	if err != nil {
		_log(nil, "Connection to %v failed - %v", host, err)
		return
	}
	defer conn.Close()

	connIP, _, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		_log(nil, "Failed to parse local addr - this should not happen: %v", err)
		return
	}

	_log(nil, "Sending ContactMe to %v; %v:%v\n", host, connIP, listenPort)

	client := http.Client{Transport: &http.Transport{Dial: connDialer{conn}.Dial}}
	url := fmt.Sprintf("http://%v:8085/contact_dls.html/ContactDLS", host)
	body := strings.NewReader(fmt.Sprintf("ContactMe=true&dls_ip_addr=%v&dls_ip_port=%v", connIP, listenPort))

	response, err := client.Post(url, "application/x-www-form-urlencoded", body)

	if err != nil {
		_log(nil, "ContactMe for %v failed: %v", host, err)
	} else if response.StatusCode != 204 {
		_log(nil, "Unexpected response from %v for ContactMe: %v", host, response.Status)
	} else {
		_log(nil, "ContactMe successfully sent to %v\n", host)
	}
}

func timerFunc(managedPhones []item, manageInterval time.Duration, listenPort string) {
	ticker := time.NewTicker(manageInterval)
	defer ticker.Stop()

	_log(nil, "Sending initial ContactMe to %v phones\n", len(managedPhones))
	for {
		for _, phone := range managedPhones {
			sendContactMe(listenPort, phone.Value)

			// XXX optionally wait between individual phones
			// this makes the log cleaner for debugging,
			// but does not serve any further purpose
			time.Sleep(5 * time.Second)
		}

		<-ticker.C
		_log(nil, "Ticker elapsed; sending ContactMe to %v phones\n", len(managedPhones))
	}
}

func main() {
	phoneState = make(map[string]*phoneDesc)

	config := itemsFromFile(nil, confSrv)

	listenIP := requireItem(config, "listen-ip").Value
	listenPort := requireItem(config, "listen-port").Value

	tlsCert := requireItem(config, "tls-cert-file").Value
	tlsKey := requireItem(config, "tls-key-file").Value

	managedPhones := make([]item, 0)
	for _, item := range filterItemList(config, "managed-phones", true) {
		managedPhones = append(managedPhones, item)
	}

	manageIntervalStr := requireItem(config, "manage-interval").Value
	manageInterval, err := time.ParseDuration(manageIntervalStr)
	if err != nil {
		_log(nil, "Failed to parse manage-interval '%v'\n", manageIntervalStr)
		os.Exit(1)
		panic("")

	}
	go timerFunc(managedPhones, manageInterval, listenPort)

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.SetTrustedProxies(nil)

	router.GET("/file/:file", getFile)
	router.POST("/DeploymentService/LoginService", postLoginService)

	router.RunTLS(fmt.Sprintf("%v:%v", listenIP, listenPort), tlsCert, tlsKey)
}
