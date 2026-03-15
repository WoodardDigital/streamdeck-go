// streamdeck-helper is a small privileged daemon that runs as root and executes
// whitelisted commands on behalf of the unprivileged streamdeck-go process.
//
// It communicates over a Unix socket. The main process sends a JSON request
// containing only a command name; the helper validates it against a root-owned
// whitelist before running anything. Arbitrary shell commands are never accepted.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

const (
	socketPath    = "/run/streamdeck-go/helper.sock"
	whitelistPath = "/etc/streamdeck-go/privileged.yaml"
	socketMode    = 0660
)

type whitelist struct {
	Commands map[string]string `yaml:"commands"`
}

type request struct {
	Command string `json:"command"`
}

type response struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func main() {
	// Must run as root.
	if os.Getuid() != 0 {
		log.Fatal("streamdeck-helper must run as root (install as a system service)")
	}

	wl, err := loadWhitelist(whitelistPath)
	if err != nil {
		log.Fatalf("load whitelist %q: %v", whitelistPath, err)
	}
	log.Printf("loaded %d whitelisted commands from %s", len(wl.Commands), whitelistPath)

	if err := os.MkdirAll(filepath.Dir(socketPath), 0755); err != nil {
		log.Fatalf("create socket dir: %v", err)
	}
	// Remove stale socket from a previous run.
	_ = os.Remove(socketPath)

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("listen on %s: %v", socketPath, err)
	}
	defer ln.Close()

	if err := os.Chmod(socketPath, socketMode); err != nil {
		log.Fatalf("chmod socket: %v", err)
	}
	// Chown the socket to root:streamdeck so group members can connect.
	// Without this the socket is root:root and only root can reach the helper.
	if grp, err := user.LookupGroup("streamdeck"); err != nil {
		log.Fatalf("group 'streamdeck' not found — run 'make install-helper' first: %v", err)
	} else if gid, err := strconv.Atoi(grp.Gid); err != nil {
		log.Fatalf("invalid gid %q: %v", grp.Gid, err)
	} else if err := os.Lchown(socketPath, 0, gid); err != nil {
		log.Fatalf("chown socket: %v", err)
	}

	log.Printf("listening on %s (group: streamdeck)", socketPath)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			continue
		}
		go handle(conn, wl)
	}
}

func handle(conn net.Conn, wl *whitelist) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return
	}

	var req request
	if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
		send(conn, response{Error: "invalid request"})
		return
	}

	if req.Command == "" {
		send(conn, response{Error: "empty command name"})
		return
	}

	shell, ok := wl.Commands[req.Command]
	if !ok {
		log.Printf("REJECTED unknown command %q", req.Command)
		send(conn, response{Error: fmt.Sprintf("unknown command %q — add it to %s", req.Command, whitelistPath)})
		return
	}

	log.Printf("running %q → %q", req.Command, shell)
	out, err := exec.Command("sh", "-c", shell).CombinedOutput()
	if err != nil {
		msg := fmt.Sprintf("%v", err)
		if len(out) > 0 {
			msg = fmt.Sprintf("%v: %s", err, out)
		}
		log.Printf("command %q failed: %s", req.Command, msg)
		send(conn, response{Error: msg})
		return
	}

	send(conn, response{OK: true})
}

func send(conn net.Conn, resp response) {
	data, _ := json.Marshal(resp)
	_, _ = conn.Write(append(data, '\n'))
}

func loadWhitelist(path string) (*whitelist, error) {
	// Verify the file is owned by root and not world-writable.
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode().Perm()&0002 != 0 {
		return nil, fmt.Errorf("%s is world-writable — refusing to load (chmod o-w %s)", path, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var wl whitelist
	if err := yaml.Unmarshal(data, &wl); err != nil {
		return nil, err
	}
	if wl.Commands == nil {
		wl.Commands = map[string]string{}
	}
	return &wl, nil
}
