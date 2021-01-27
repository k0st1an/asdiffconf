package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"golang.org/x/sys/unix"
)

var (
	host    string
	port    uint
	context string
)

type config struct {
	Host    string
	Options []option
}

type option struct {
	Key   string
	Value string
}

func main() {
	log.SetFlags(log.Ldate | log.Lmicroseconds | log.Lshortfile)
	flag.StringVar(&host, "host", "127.0.0.1", "remote host")
	flag.UintVar(&port, "port", 3000, "remote port")
	flag.StringVar(&context, "context", "", "context of config, example: service, network, namespace;id=<name>, security")
	flag.Parse()

	configs := getConfigs()
	filtered := map[string]string{}

	for _, hostConf := range configs {
		for _, confOpt := range hostConf.Options {
			if isDiff(hostConf.Host, confOpt.Key, confOpt.Value, configs) {
				filtered[confOpt.Key] = lookUpValueOfKey(confOpt.Key, configs)
			}
		}
	}

	width := getTerminalWidth() - 45

	for key, value := range filtered {
		if len(value) > int(width) {
			fmt.Printf("%45s: ", key)
			firstLine := true
			for _, item := range strings.Split(value, ";") {
				if firstLine {
					fmt.Printf("%s\n", strings.TrimSpace(item))
					firstLine = false
				}
				fmt.Printf("%s%s\n", getSpaces(47), strings.TrimSpace(item))
			}
			continue
		}
		fmt.Printf("%45s: %s\n", strings.TrimSpace(key), strings.TrimSpace(value))
	}
}

func isDiff(refHost, refKey, refValue string, configs []config) bool {
	for _, host := range configs {
		if refHost != host.Host {
			for _, opts := range host.Options {
				if opts.Key == refKey && opts.Value != refValue {
					return true
				}
			}
		}
	}

	return false
}

func getSpaces(n int) string {
	s := ""
	for i := 0; i < n; i++ {
		s = s + " "
	}
	return s
}

func getTerminalWidth() uint16 {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		log.Fatalln(os.NewSyscallError("GetWinsize", err))
	}

	return ws.Col
}

func getConfigs() []config {
	var cmd string
	if context == "" {
		cmd = fmt.Sprintf("asadm -h %s -p %d -e \"asinfo -v 'get-config:'\"", host, port)
	} else {
		cmd = fmt.Sprintf("asadm -h %s -p %d -e \"asinfo -v 'get-config:context=%s'\"", host, port, context)
	}

	bytes, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		log.Fatalln(string(bytes))
	}

	skipFirstLine := 2
	lines := 1
	var configs []config

	scanner := bufio.NewScanner(strings.NewReader(string(bytes)))
	for scanner.Scan() {
		if lines <= skipFirstLine || len(scanner.Text()) == 0 {
			lines++
			continue
		}

		var hostConf config
		hostConf.Host = lookUpHost(scanner.Text())

		scanner.Scan() // read next line

		hostConf.Options = lookUpConfig(scanner.Text())
		configs = append(configs, hostConf)
	}

	return configs
}

// Parse strings:
//   10.99.68.124:3000 (10.99.68.124) returned:
//   eu-a15:3000 (10.99.68.124) returned:
func lookUpHost(str string) string {
	rIP := regexp.MustCompile(`\((?P<ip>\d+\.\d+\.\d+\.\d+)\)`)
	match := rIP.FindStringSubmatch(str)
	if len(match) == 2 {
		return ipToHostName(match[1])
	}

	return str
}

func lookUpConfig(str string) []option {
	var options []option
	for _, item := range strings.Split(str, ";") {
		if len(strings.Split(item, "=")) != 2 {
			log.Fatalln("Incorrect output from AS?")
		}
		options = append(options, option{Key: strings.TrimSpace(strings.Split(item, "=")[0]), Value: strings.TrimSpace((strings.Split(item, "=")[1]))})
	}

	return options
}

func ipToHostName(ip string) string {
	addr, err := net.LookupAddr(ip)
	if err != nil {
		return ip
	}

	return addr[0] // TODO: fix it
}

func lookUpValueOfKey(key string, configs []config) string {
	var keyResult string

	for _, host := range configs {
		for _, opt := range host.Options {
			if opt.Key == key {
				keyResult = fmt.Sprintf("%s; %s = %s", keyResult, host.Host, opt.Value)
			}
		}
	}
	return keyResult[2:]
}
