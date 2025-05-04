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
	"golang.org/x/term"
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

func clearScreen() {
	fmt.Print("\033[H\033[2J")
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
	clearScreen()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	banner := `
	╔══════════════════════════════════════════════════╗
	║           🛰️  ChatP2P - Cliente Terminal          ║
	║           Desarrollado por Miguel en Go 💬        ║
	╚══════════════════════════════════════════════════╝
	`

	color.Yellow(banner)
	color.Cyan("💡 Este cliente te permite conectarte a otros usuarios en tiempo real.\n")
	color.Green("➡️  Pasos iniciales:")
	fmt.Println("   1. Usa /register para crear una cuenta.")
	fmt.Println("   2. Luego usa /login para ingresar.")
	fmt.Println("   3. Cuando estés dentro, escribe /help para ver más comandos.")

	fmt.Print("🔌 Servidor WebSocket [localhost:8080]: ")
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
			fmt.Print("🆕 Nombre de usuario: ")
			userInput, _ := reader.ReadString('\n')
			user := strings.TrimSpace(userInput)

			fmt.Print("🔐 Contraseña (oculta): ")
			passBytes, _ := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			pass := strings.TrimSpace(string(passBytes))

			ws, _, err = websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				color.Red("Error conectando: %v", err)
				return
			}
			self = user
			ws.WriteJSON(map[string]string{
				"type":     "register",
				"username": user,
				"password": pass,
			})

			var resp SignalMsg
			ws.ReadJSON(&resp)
			if resp.Type != "register-success" {
				color.Red("❌ Registro fallido: %s", resp.Message)
				ws.Close()
				continue
			}
			color.Green("✅ Registro exitoso. Ahora inicia sesión con /login.")

		case "/login":
			fmt.Print("👤 Usuario: ")
			userInput, _ := reader.ReadString('\n')
			user := strings.TrimSpace(userInput)

			fmt.Print("🔐 Contraseña (oculta): ")
			passBytes, _ := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			pass := strings.TrimSpace(string(passBytes))

			ws, _, err = websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				color.Red("Error conectando: %v", err)
				return
			}
			self = user
			ws.WriteJSON(map[string]string{
				"type":     "login",
				"username": user,
				"password": pass,
			})

			var resp SignalMsg
			ws.ReadJSON(&resp)
			if resp.Type != "login-success" {
				color.Red("❌ Login fallido: %s", resp.Message)
				ws.Close()
				continue
			}
			clearScreen()
			color.Green("✅ Login exitoso. ¡Bienvenido, %s!", self)
			goto ChatLoop

		case "/exit":
			return

		default:
			color.Red("Primero debes /register o /login.")
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
		case "/help", "/menu", "7":
			printHelp()

		case "/exit", "8":
			color.Red("\U0001F44B Bye!")
			return

		case "/list", "3":
			ws.WriteJSON(SignalMsg{Type: "list-users"})
			users := <-userListCh
			color.Cyan("\U0001F465 Usuarios online:")
			for _, u := range users {
				fmt.Println(" -", u)
			}

		case "/to", "4":
			if len(parts) < 2 {
				color.Red("Uso: /to <usuario>")
			} else {
				currentTo = parts[1]
				color.Green("\u2705 Destinatario seleccionado: %s", currentTo)
			}

		case "/p2p", "5":
			if len(parts) < 2 {
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
				ws.WriteJSON(Message{
					From:      self,
					To:        user,
					Content:   msg,
					Type:      "text",
					Timestamp: time.Now(),
				})
			} else {
				color.Red("\u274C Comando desconocido. Usa /help o escribe 7 para ver el men\u00fa.")
			}
		}
		rl.SetPrompt(makePrompt())
	}
}
