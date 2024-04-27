package main

import (
	"flag"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

func main() {
	var port int
	flag.IntVar(&port, "port", 8080, "port")

	var verbose bool
	flag.BoolVar(&verbose, "verbose", false, "set verbose")

	var listenIface string
	flag.StringVar(&listenIface, "host", "127.0.0.1", "listen interface")

	var netrcPath string
	flag.StringVar(&netrcPath, "netrc", "~/.netrc", "netrc path")

	flag.Parse()

	if strings.HasPrefix(netrcPath, "~/") {
		usr, err := user.Current()
		if err != nil {
			log.Panic(err)
		}
		dir := usr.HomeDir
		netrcPath = filepath.Join(dir, netrcPath[2:])
	}

	cmd := flag.Args()
	
	success, err := Run(Config{
		port: port,
		verbose: verbose,
		listenIface: listenIface,
		netrcPath: netrcPath,
		cmd: cmd,
	})
	if err != nil {
		log.Panic(err)
	}

	if success {
		os.Exit(0)
	} else {
		os.Exit(1)
	}
}
