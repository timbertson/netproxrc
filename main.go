package main

import (
	"flag"
	"log"
	"os"
)

func main() {
	var port int
	flag.IntVar(&port, "port", 0, "port (0 will use a random port)")

	var verbose bool
	flag.BoolVar(&verbose, "verbose", false, "set verbose")

	var listenIface string
	flag.StringVar(&listenIface, "host", "127.0.0.1", "listen interface")

	var netrcPath string
	flag.StringVar(&netrcPath, "netrc", "~/.netrc", "netrc path")

	flag.Parse()

	cmd := flag.Args()
	
	info := func(msg string, argv ...interface{}) {
		if verbose {
			log.Printf("INFO: "+msg, argv...)
		}
	}

	success, err := Run(Config{
		port: port,
		verbose: verbose,
		listenIface: listenIface,
		netrcPath: netrcPath,
		cmd: cmd,
		info: info,
		suppressPrintf: false,
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
