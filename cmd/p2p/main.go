package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

type SignalMsg struct {
	Type       string   `json:"type"`
	Username   string   `json:"from,omitempty"`
	To         string   `json:"to,omitempty"`
	SignalType string   `json:"signalType,omitempty"`
	Address    string   `json:"address,omitempty"`
	Users      []string `json:"users,omitempty"`
}

var (
	ws   *websocket.Conn
	self string
)

func startP2P(peer string) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		fmt.Println("Error listener P2P:", err)
		return
	}
	addr := ln.Addr().String()
	offer := SignalMsg{Type: "signal", SignalType: "offer", To: peer, Username: self, Address: addr}
	ws.WriteJSON(offer)

	conn, err := ln.Accept()
	ln.Close()
	if err != nil {
		fmt.Println("Error aceptar P2P:", err)
		return
	}
	fmt.Println("P2P conectado con", peer)
	go chatReceive(conn, peer)
	chatSend(conn)
}

func chatReceive(c net.Conn, peer string) {
	r := bufio.NewReader(c)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			fmt.Println("Cierre de P2P.")
			os.Exit(0)
		}
		fmt.Printf("[%s]: %s", peer, line)
	}
}

func chatSend(c net.Conn) {
	w := bufio.NewWriter(c)
	scanCmd := bufio.NewScanner(os.Stdin)
	for scanCmd.Scan() {
		w.WriteString(scanCmd.Text() + "\n")
		w.Flush()
	}
}

func main() {
	scanLogin := bufio.NewScanner(os.Stdin)
	fmt.Print("Usuario: ")
	scanLogin.Scan()
	self = scanLogin.Text()

	var err error
	ws, _, err = websocket.DefaultDialer.Dial("ws://localhost:8080/ws", nil)
	if err != nil {
		fmt.Println("No conectar:", err)
		return
	}
	defer ws.Close()

	ws.WriteJSON(map[string]string{"type": "login", "username": self})
	var resp SignalMsg
	ws.ReadJSON(&resp)
	if resp.Type != "login-success" {
		fmt.Println("Login fallido.")
		return
	}
	fmt.Println("Login OK. /list para ver peers, /p2p <user> para conectar.")

	// Lector WS
	go func() {
		scanSignal := bufio.NewScanner(os.Stdin) // renombrado para evitar shadow
		for {
			var msg SignalMsg
			if err := ws.ReadJSON(&msg); err != nil {
				fmt.Println("WS err:", err)
				os.Exit(1)
			}
			switch msg.Type {
			case "user-list":
				fmt.Println("Peers:", msg.Users)
			case "signal":
				if msg.SignalType == "offer" {
					fmt.Printf("%s ofrece P2P (%s). Aceptar? (s/n): ", msg.Username, msg.Address)
					scanSignal.Scan()
					if strings.TrimSpace(scanSignal.Text()) == "s" {
						conn, err := net.Dial("tcp", msg.Address)
						if err == nil {
							fmt.Println("Conectado a", msg.Username)
							go chatReceive(conn, msg.Username)
							chatSend(conn)
						}
					}
				}
			}
		}
	}()

	// CLI loop
	scanCLI := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		scanCLI.Scan()
		line := scanCLI.Text()
		if line == "/list" {
			ws.WriteJSON(map[string]string{"type": "list-users"})
		} else if strings.HasPrefix(line, "/p2p ") {
			peer := strings.TrimSpace(strings.TrimPrefix(line, "/p2p "))
			startP2P(peer)
		}
	}
}
