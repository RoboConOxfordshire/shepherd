package ws

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"os/exec"

	"github.com/gorilla/websocket"
)

type PylsProxy struct {
	robotLibPath string
}

func NewPylsProxy(robotLibPath string) *PylsProxy {
	return &PylsProxy{robotLibPath: robotLibPath}
}

func (p *PylsProxy) Handle(conn *websocket.Conn) {
	defer conn.Close()

	cmd := exec.Command("python3", "-m", "pyls", "-v")
	cmd.Env = append(cmd.Environ(), "PYTHONPATH="+p.robotLibPath)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("pyls stdin pipe: %v", err)
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("pyls stdout pipe: %v", err)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("pyls start: %v", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		defer stdout.Close()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			msg := scanner.Bytes()
			cp := make([]byte, len(msg))
			copy(cp, msg)
			if err := conn.WriteMessage(websocket.TextMessage, cp); err != nil {
				cancel()
				return
			}
		}
		cancel()
	}()

	go func() {
		defer stdin.Close()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			_, msg, err := conn.ReadMessage()
			if err != nil {
				cmd.Process.Kill()
				cancel()
				return
			}
			var parsed any
			if err := json.Unmarshal(msg, &parsed); err == nil {
				serialized, _ := json.Marshal(parsed)
				stdin.Write(serialized)
			}
		}
	}()

	<-ctx.Done()
	io.Copy(io.Discard, stdout)
	cmd.Wait()
}
