package main

import (
	"context"
	"encoding/base64"

	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/bgentry/go-netrc/netrc"
	"github.com/elazarl/goproxy"
)

type Config struct {
	port int
	verbose bool
	listenIface string
	netrcPath string
	cmd []string
}

func Run(config Config) (bool, error) {
	return WithProc(config, func(proc *exec.Cmd) (bool, error) {
		err := proc.Start()
		if err != nil {
			return false, err
		}
		err = proc.Wait()
		if err != nil {
			return false, nil // don't report this error, just exit
		}
		return true, nil
	})
}

func WithProc(config Config, block func(*exec.Cmd) (bool, error)) (bool, error) {
	verbose := config.verbose
	listenIface := config.listenIface
	netrcPath := config.netrcPath
	cmd := config.cmd

	info := func(msg string, argv ...interface{}) {
		if verbose {
			log.Printf("INFO: "+msg, argv...)
		}
	}

	info("Loading %s", netrcPath)
	netrcFile, err := netrc.ParseFile(netrcPath)
	if err != nil {
		return false, err
	}

	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = verbose

	// disable InsecureSkipVerify which is enabled by goproxy for some reason
	tlsConfig := proxy.Tr.TLSClientConfig.Clone()
	tlsConfig.InsecureSkipVerify = false
	proxy.Tr.TLSClientConfig = tlsConfig

	var mitmAuthHosts goproxy.FuncHttpsHandler = func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		hostname := strings.Split(host, ":")[0] // remove port
		// only MITM if it's an authenticated host, to minimuise intrusion
		if netrcFile.FindMachine(hostname) != nil {
			info("MitmConnect: %s", hostname)
			return goproxy.MitmConnect, host
		} else {
			info("OkConnect: %s", hostname)
			return goproxy.OkConnect, host
		}
	}
	proxy.OnRequest().HandleConnect(mitmAuthHosts)

	proxy.OnRequest().DoFunc(
		func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			existingAuth := r.Header.Get("authorization")
			if existingAuth != "" {
				return r, nil
			}

			machine := netrcFile.FindMachine(r.Host)
			if machine != nil {
				info("Injecting auth for %s", r.Host)
				loginStr := fmt.Sprintf("%s:%s", machine.Login, machine.Password)
				loginB64 := base64.StdEncoding.EncodeToString([]byte(loginStr))
				r.Header.Add("authorization", fmt.Sprintf("Basic %s", loginB64))
			}
			return r, nil
		})
	addr := fmt.Sprintf("%s:%d", listenIface, config.port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// done := make(chan struct{})

	server := &http.Server{
		Addr:    addr,
		Handler: proxy,
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	listenAddr := listener.Addr().String()
	log.Printf("Listening on: %s", listenAddr)

	// TCP port is listening, spawn server in background
	go func() {
		err := server.Serve(listener)
		// server should never die
		log.Fatal(err)
	}()

	cacerts := fmt.Sprintf("# netproxrc self-signed root\n\n%s", string(goproxy.CA_CERT))

	systemCertPath := os.Getenv("CURL_CA_BUNDLE")
	if systemCertPath != "" {
		info("merging certificates with system bundle: %s", systemCertPath)
		systemCerts, err := os.ReadFile(systemCertPath)
		if err != nil {
			return false, err
		}
		cacerts = fmt.Sprintf("%s\n\n%s\n", systemCerts, cacerts)
	}

	// CA bundle should be world-readable since builders may run as a different user
	// NOTE: do not use os.TempDir() on MacOS, it plays shenanigans to make that unreadable to other users
	certPath := "/tmp/netproxrc-cert.pem"
	err = os.WriteFile(certPath, []byte(cacerts), 0644)
	if err != nil {
		return false, err
	}
	info("Wrote CA cert to %s", certPath)

	err = os.Chmod(certPath, 0644)
	if err != nil {
		return false, err
	}

	additionalEnv := (func() []string {
		newEnv := []string{}

		http_proxy := fmt.Sprintf("http://%s", listenAddr)

		for _, key := range []string{"https_proxy"} {
			newEnv = append(newEnv, fmt.Sprintf("%s=%s", key, http_proxy))
		}

		for _, key := range []string{
			"NIX_SSL_CERT_FILE",       // nix aware wrappers
			"CURL_CA_BUNDLE",          // curl
			"GIT_SSL_CAINFO",          // git
			"NIX_GIT_SSL_CAINFO",      // nix-aware git
			"SSL_CERT_FILE",           // openssl
			"SYSTEM_CERTIFICATE_PATH", // haskell x509
		} {
			newEnv = append(newEnv, fmt.Sprintf("%s=%s", key, certPath))
		}

		// set env vars in nix impure-env as well
		nixConfigStr := os.Getenv("NIX_CONFIG")
		var nixConfig []string
		if nixConfigStr == "" {
			nixConfig = []string{}
		} else {
			nixConfig = strings.Split("\n", nixConfigStr)
		}
		nixConfig = append(nixConfig, fmt.Sprintf("impure-env = %s", strings.Join(newEnv, " ")))
		newEnv = append(newEnv, fmt.Sprintf("NIX_CONFIG=%s", strings.Join(nixConfig, "\n")))

		for _, envvar := range newEnv {
			info("+ export %s", strings.ReplaceAll(envvar, "\n", "\\\n  "))
		}
		return newEnv
	})()

	// run command in foreground (or wait for TERM)
	if len(cmd) == 0 {
		log.Print("Press ctrl+c to terminate")
		select {}
	} else {
		exe := cmd[0]
		args := cmd[1:]
		proc := exec.Command(exe, args...)
		proc.Env = append(os.Environ(), additionalEnv...)

		info(" + %v", cmd)

		proc.Stdin = os.Stdin
		proc.Stdout = os.Stdout
		proc.Stderr = os.Stderr
		return block(proc)
	}
	return true, nil
}
