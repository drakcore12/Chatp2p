package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/chzyer/readline"
	"github.com/fatih/color"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

type SignalMsg struct {
	Type       string                     `json:"type"`
	Username   string                     `json:"from,omitempty"`
	To         string                     `json:"to,omitempty"`
	SignalType string                     `json:"signalType,omitempty"` // "offer","answer","ice"
	SDP        *webrtc.SessionDescription `json:"sdp,omitempty"`
	ICE        *webrtc.ICECandidateInit   `json:"ice,omitempty"`
	Users      []string                   `json:"users,omitempty"`
	Message    string                     `json:"message,omitempty"`
}

type Message struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Content   string    `json:"content"`
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
}

var (
	ws          *websocket.Conn
	self        string
	currentTo   string
	activeUsers []string
	rl          *readline.Instance
	pc          *webrtc.PeerConnection
	dc          *webrtc.DataChannel
	completer   *readline.PrefixCompleter
)

func initCompleter() {
	completer = readline.NewPrefixCompleter(
		readline.PcItem("/help"),
		readline.PcItem("/register"),
		readline.PcItem("/login"),
		readline.PcItem("/list"),
		readline.PcItem("/to"),
		readline.PcItem("/p2p"),
		readline.PcItem("@", readline.PcItemDynamic(func(string) []string {
			return activeUsers
		})),
	)
}

func makePrompt() string {
	if currentTo == "" {
		return color.GreenString("%s> ", self)
	}
	return color.GreenString("%s@%s> ", self, currentTo)
}

func printHelp() {
	color.Magenta("Comandos disponibles:")
	fmt.Println("  /help                  - Muestra esta ayuda")
	fmt.Println("  /register <user> <pwd> - Registra nueva cuenta")
	fmt.Println("  /login <user> <pwd>    - Inicia sesión")
	fmt.Println("  /list                  - Lista usuarios conectados")
	fmt.Println("  /to <user>             - Selecciona destinatario")
	fmt.Println("  /p2p <user>            - Inicia chat P2P directo (WebRTC)")
	fmt.Println("  @user <msg>            - Envía mensaje via servidor")
	fmt.Println("  /exit                  - Salir")
}

func setupWebRTC() error {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
			{
				URLs:       []string{"turn:midominio.com:3478?transport=udp"},
				Username:   "usuarioTURN",
				Credential: "claveTURN",
			},
			{
				URLs:       []string{"turn:midominio.com:3478?transport=tcp"},
				Username:   "usuarioTURN",
				Credential: "claveTURN",
			},
			{
				URLs:       []string{"turn:midominio.com:5349?transport=tcp"},
				Username:   "usuarioTURN",
				Credential: "claveTURN",
			},
		},
	}

	var err error
	pc, err = webrtc.NewPeerConnection(config)
	if err != nil {
		return err
	}

	// FIX: tomar la dirección del ICECandidateInit
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		cand := c.ToJSON()
		ws.WriteJSON(SignalMsg{
			Type:       "signal",
			SignalType: "ice",
			To:         currentTo,
			ICE:        &cand,
		})
	})

	dc, err = pc.CreateDataChannel("chat", nil)
	if err != nil {
		return err
	}
	dc.OnOpen(func() {
		color.Green(">> Canal P2P abierto con %s", currentTo)
	})
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		fmt.Printf("%s %s\n",
			color.CyanString("["+currentTo+"]>"),
			string(msg.Data),
		)
	})

	return nil
}

func handleSignal(s SignalMsg) {
	switch s.SignalType {
	case "offer":
		pc.SetRemoteDescription(*s.SDP)
		answer, err := pc.CreateAnswer(nil)
		if err != nil {
			log.Println("CreateAnswer error:", err)
			return
		}
		pc.SetLocalDescription(answer)
		ws.WriteJSON(SignalMsg{
			Type:       "signal",
			SignalType: "answer",
			To:         s.Username,
			SDP:        &answer,
		})

	case "answer":
		pc.SetRemoteDescription(*s.SDP)

	case "ice":
		if s.ICE != nil {
			pc.AddICECandidate(*s.ICE)
		}
	}
}

func main() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	fmt.Print("Servidor WS [localhost:8080]: ")
	reader := bufio.NewReader(os.Stdin)
	srv, _ := reader.ReadString('\n')
	srv = strings.TrimSpace(srv)
	if srv == "" {
		srv = "localhost:8080"
	}
	wsURL := fmt.Sprintf("ws://%s/ws", srv)

	initCompleter()
	var err error
	rl, err = readline.NewEx(&readline.Config{
		Prompt:          makePrompt(),
		HistoryFile:     "/tmp/chat_history.log",
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "/exit",
	})
	if err != nil {
		log.Fatalln("readline init:", err)
	}
	defer rl.Close()

	// Registro/Login
	for {
		line, err := rl.Readline()
		if err != nil {
			return
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		switch parts[0] {
		case "/register":
			if len(parts) != 3 {
				color.Red("Uso: /register <user> <pwd>")
				continue
			}
			ws, _, err = websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				color.Red("Error connect: %v", err)
				return
			}
			self = parts[1]
			ws.WriteJSON(SignalMsg{Type: "register", Username: self, Message: parts[2]})
			var resp SignalMsg
			ws.ReadJSON(&resp)
			if resp.Type != "register-success" {
				color.Red("Registro fallido: %s", resp.Message)
				ws.Close()
				continue
			}
			color.Green("Registro exitoso, ahora /login.")

		case "/login":
			if len(parts) != 3 {
				color.Red("Uso: /login <user> <pwd>")
				continue
			}
			ws, _, err = websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				color.Red("Error connect: %v", err)
				return
			}
			self = parts[1]
			ws.WriteJSON(SignalMsg{Type: "login", Username: self, Message: parts[2]})
			var resp SignalMsg
			ws.ReadJSON(&resp)
			if resp.Type != "login-success" {
				color.Red("Login fallido: %s", resp.Message)
				ws.Close()
				continue
			}
			color.Green("Login exitoso. ¡Bienvenido, %s!", self)
			goto ChatLoop

		case "/exit":
			return

		default:
			color.Red("Primero registra o loguea. Usa /register o /login.")
		}
		rl.SetPrompt(makePrompt())
	}

ChatLoop:
	if err := setupWebRTC(); err != nil {
		log.Fatalln("WebRTC setup:", err)
	}

	userListCh := make(chan []string)
	go func() {
		for {
			_, raw, err := ws.ReadMessage()
			if err != nil {
				color.Red("WS cerrado: %v", err)
				os.Exit(1)
			}
			var base struct{ Type string }
			json.Unmarshal(raw, &base)
			switch base.Type {
			case "user-list":
				var ul struct {
					Type  string   `json:"type"`
					Users []string `json:"users"`
				}
				json.Unmarshal(raw, &ul)
				activeUsers = ul.Users
				userListCh <- ul.Users

			case "text":
				var m Message
				json.Unmarshal(raw, &m)
				fmt.Printf("[%s]> %s\n", m.From, m.Content)

			case "signal":
				var s SignalMsg
				json.Unmarshal(raw, &s)
				handleSignal(s)

			case "error":
				var e struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				}
				json.Unmarshal(raw, &e)
				color.Red("Error: %s", e.Message)
			}
		}
	}()

	printHelp()
	for {
		line, err := rl.Readline()
		if err != nil {
			return
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		switch parts[0] {
		case "/help":
			printHelp()
		case "/exit":
			color.Red("Bye!")
			return
		case "/list":
			ws.WriteJSON(SignalMsg{Type: "list-users"})
			users := <-userListCh
			color.Cyan("Usuarios online:")
			for _, u := range users {
				fmt.Println(" -", u)
			}
		case "/to":
			if len(parts) != 2 {
				color.Red("Uso: /to <usuario>")
			} else {
				currentTo = parts[1]
				color.Green("Destinatario: %s", currentTo)
			}
		case "/p2p":
			if len(parts) != 2 {
				color.Red("Uso: /p2p <usuario>")
			} else {
				currentTo = parts[1]
				offer, err := pc.CreateOffer(nil)
				if err != nil {
					color.Red("Error CreateOffer: %v", err)
					continue
				}
				pc.SetLocalDescription(offer)
				ws.WriteJSON(SignalMsg{Type: "signal", SignalType: "offer", To: currentTo, SDP: &offer})
			}
		default:
			if strings.HasPrefix(parts[0], "@") && len(parts) > 1 {
				user := strings.TrimPrefix(parts[0], "@")
				msg := strings.Join(parts[1:], " ")
				ws.WriteJSON(Message{From: self, To: user, Content: msg, Type: "text", Timestamp: time.Now()})
			} else {
				color.Red("Comando desconocido. Usa /help.")
			}
		}
		rl.SetPrompt(makePrompt())
	}
}
