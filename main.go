package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"

	"gopkg.in/ini.v1"
)

const OVHAPIEndpoint = "https://www.ovh.com/nic/update"

func main() {
	configFile := flag.String(
		"config",
		"./config.cfg",
		"path to the configuration file to uses")

	dryRun := flag.Bool(
		"dry",
		false,
		"do not actually configure the new DynHost")

	showVersion := flag.Bool(
		"version",
		false,
		"show the version of this software")

	flag.Parse()

	if *showVersion {
		println("go-dynhost 1.0.0")
		return
	}

	cfg, err := ini.Load(*configFile)
	if err != nil {
		log.Fatalf("Could not open %s: %v", *configFile, err)
	}

	publicIP, err := getPublicIPv4()
	if err != nil {
		log.Fatalf("Could not get my public IPv4 address: %v", err)
	}

	log.Printf("Public IP: %s", publicIP.String())

	cfgSection := cfg.Section("ovh")

	username := cfgSection.Key("username").String()
	if username == "" {
		log.Fatalf("%s: username cannot be empty", *configFile)
	}

	password := cfgSection.Key("password").String()
	if password == "" {
		log.Fatalf("%s: password cannot be empty", *configFile)
	}

	hostname := cfgSection.Key("hostname").String()
	if hostname == "" {
		log.Fatalf("%s: hostname cannot be empty", *configFile)
	}

	currentDynHostIP, err := getDynHostValue(hostname)
	if err != nil {
		log.Fatalf("Could not get the current DynHost value: %v", err)
	}

	log.Printf("Current DynHost value: %s", currentDynHostIP.String())

	if bytes.Compare(publicIP, currentDynHostIP) == 0 {
		log.Print("The current DynHost record is up-to-date; exiting.")
		return
	}

	if *dryRun {
		log.Print("Dry run; exiting.")
		return
	}

	if err := updateDynHost(username, password, hostname, publicIP); err != nil {
		log.Fatalf("Could not update the DynHost record: %v", err)
	}
}

func getDynHostValue(hostname string) (net.IP, error) {
	addrs, err := net.LookupIP(hostname)
	if err != nil {
		return nil, err
	}

	for _, a := range addrs {
		if a.To4() != nil {
			return a, nil
		}
	}

	return nil, errors.New("no IPv4 found")
}

func getPublicIPv4() (net.IP, error) {
	errIP := net.IPv4zero

	res, err := http.Get("https://api.ipify.org")
	if err != nil {
		return errIP, err
	}
	defer res.Body.Close()

	resCode := res.StatusCode

	if resCode != http.StatusOK {
		return errIP, fmt.Errorf("returned %d", resCode)
	}

	ipStrBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errIP, fmt.Errorf("could not read the response: %v", err)
	}

	return net.ParseIP(string(ipStrBytes)), nil
}

func updateDynHost(username, password, hostname string, address net.IP) error {
	req, err := http.NewRequest(http.MethodGet, OVHAPIEndpoint, nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(username, password)

	q := req.URL.Query()
	q.Add("system", "dyndns")
	q.Add("hostname", hostname)
	q.Add("myip", address.String())

	req.URL.RawQuery = q.Encode()

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("the OVH API replied %s", res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("could not read the response body: %v", err)
	}

	bodyStr := strings.Split(string(body), " ")



	if len(bodyStr) < 1 || bodyStr[0] != "good" {
		return fmt.Errorf("response body: %s", bodyStr)
	}

	return nil
}