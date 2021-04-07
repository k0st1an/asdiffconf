package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"golang.org/x/sys/unix"
)

var (
	host      string
	port      uint
	namespace string
	version   bool
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
	flag.StringVar(&host, "H", "127.0.0.1", "remote host")
	flag.UintVar(&port, "p", 3000, "remote port")
	flag.StringVar(&namespace, "n", "", "get config of namespace")
	flag.BoolVar(&version, "v", false, "print version and exit")
	flag.Parse()

	if version {
		fmt.Println("v1.0.0")
		os.Exit(0)
	}

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
	if namespace != "" {
		cmd = fmt.Sprintf("asadm -h %s -p %d -e \"asinfo -v 'get-config:context=namespace;id=%s'\"", host, port, namespace)
	} else {
		cmd = fmt.Sprintf("asadm -h %s -p %d -e \"asinfo -v 'get-config'\"", host, port)
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
		return match[1]
	}

	return str
}

func lookUpConfig(str string) []option {
	var options []option
	for _, item := range strings.Split(str, ";") {
		if len(strings.Split(item, "=")) != 2 {
			log.Println("Incorrect output from AS?")
			log.Printf("Output: %s\n", str)
			os.Exit(1)
		}
		options = append(options, option{Key: strings.TrimSpace(strings.Split(item, "=")[0]), Value: strings.TrimSpace((strings.Split(item, "=")[1]))})
	}

	return options
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
