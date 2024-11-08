package http

import (
	"bufio"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/versioneer-tech/package-r/runner"
)

const (
	WSWriteDeadline = 10 * time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var (
	cmdNotAllowed = []byte("Command not allowed.")
)

//nolint:unparam
func wsErr(ws *websocket.Conn, r *http.Request, status int, err error) {
	txt := http.StatusText(status)
	if err != nil || status >= 400 {
		log.Printf("%s: %v %s %v", r.URL.Path, status, r.RemoteAddr, err)
	}
	if err := ws.WriteControl(websocket.CloseInternalServerErr, []byte(txt), time.Now().Add(WSWriteDeadline)); err != nil {
		log.Print(err)
	}
}

type CommandWriter interface {
	Write(data []byte) (int, error)
}

//nolint:stylecheck,revive
func HandleHttpCommand(w http.ResponseWriter, cw CommandWriter, dir, name string, arg ...string) (int, error) {
	cmd := exec.Command(name, arg...)
	cmd.Dir = dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return 0, nil
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return 0, nil
	}

	if err := cmd.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return 0, nil
	}

	s := bufio.NewScanner(io.Reader(stdout))
	for s.Scan() {
		if _, err := cw.Write(append(s.Bytes(), '\n')); err != nil {
			log.Print(err)
		}
	}

	s2 := bufio.NewScanner(io.Reader(stderr))
	for s2.Scan() {
		log.Print(string(s2.Bytes()))
	}

	if err := cmd.Wait(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	return 0, nil
}

var commandsHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if r.Header.Get("Upgrade") != "websocket" ||
		r.Header.Get("Connection") != "Upgrade" {
		raw := r.URL.Query().Get("raw")
		if raw == "" {
			return http.StatusBadRequest, nil
		}
		log.Print(d.user.Username, " -> (http) ", raw)

		command, err := runner.ParseCommand(d.settings, raw)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return 0, nil
		}

		if !d.server.EnableExec || !d.user.CanExecute(command[0]) {
			http.Error(w, string(cmdNotAllowed), http.StatusForbidden)
			return 0, nil
		}
		_, err = HandleHttpCommand(w, w, d.user.FullPath(r.URL.Path), command[0], command[1:]...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return 0, nil
		}
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer conn.Close()

	var raw string

	for {
		_, msg, err := conn.ReadMessage() //nolint:govet
		if err != nil {
			wsErr(conn, r, http.StatusInternalServerError, err)
			return 0, nil
		}

		raw = strings.TrimSpace(string(msg))
		if raw != "" {
			break
		}
	}
	log.Print(d.user.Username, " -> (ws) ", raw)

	command, err := runner.ParseCommand(d.settings, raw)
	if err != nil {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(err.Error())); err != nil { //nolint:govet
			wsErr(conn, r, http.StatusInternalServerError, err)
		}
		return 0, nil
	}

	if !d.server.EnableExec || !d.user.CanExecute(command[0]) {
		if err := conn.WriteMessage(websocket.TextMessage, cmdNotAllowed); err != nil { //nolint:govet
			wsErr(conn, r, http.StatusInternalServerError, err)
		}

		return 0, nil
	}

	cmd := exec.Command(command[0], command[1:]...) //nolint:gosec
	cmd.Dir = d.user.FullPath(r.URL.Path)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		wsErr(conn, r, http.StatusInternalServerError, err)
		return 0, nil
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		wsErr(conn, r, http.StatusInternalServerError, err)
		return 0, nil
	}

	if err := cmd.Start(); err != nil {
		wsErr(conn, r, http.StatusInternalServerError, err)
		return 0, nil
	}

	s := bufio.NewScanner(io.Reader(stdout))
	for s.Scan() {
		if err := conn.WriteMessage(websocket.TextMessage, s.Bytes()); err != nil {
			log.Print(err)
		}
	}

	s2 := bufio.NewScanner(io.Reader(stderr))
	for s2.Scan() {
		log.Print(string(s2.Bytes()))
	}

	if err := cmd.Wait(); err != nil {
		wsErr(conn, r, http.StatusInternalServerError, err)
	}

	return 0, nil
})
