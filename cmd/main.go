package main

import (
	"bufio"
	"bytes"
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
	chats       = make(map[string][]Message) // 🗂️ Almacena mensajes por usuario
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
	fmt.Println("  /chats [user]          - Muestra historial de chats (todos o por usuario)")
	fmt.Println("  /clear <user>          - Borra historial de un usuario")
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

func chatP2PLoop() {
	color.Green("🟢 Estás en modo P2P con %s. Escribe mensajes libremente.", currentTo)
	color.Yellow("✏️ Escribe '/exit' para salir del modo P2P.")

	for {
		line, err := rl.Readline()
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if line == "/exit" {
			color.Red("🚪 Saliste del modo P2P con %s.", currentTo)
			currentTo = ""
			return
		}
		if dc != nil && dc.ReadyState() == webrtc.DataChannelStateOpen {
			dc.SendText(line)
			fmt.Printf("[Tú -> %s] %s\n", currentTo, line)
		} else {
			color.Red("❌ El canal P2P no está disponible.")
			return
		}
	}
}

func handleSignal(s SignalMsg) {
	switch s.SignalType {
	case "offer":
		fmt.Printf("\n🔔 Solicitud de conexión P2P de %s. ¿Aceptar? [s/n/b] ", s.Username)
		reader := bufio.NewReader(os.Stdin)
		resp, _ := reader.ReadString('\n')
		resp = strings.ToLower(strings.TrimSpace(resp))

		switch resp {
		case "s":
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
			currentTo = s.Username
			go chatP2PLoop()

		case "n":
			color.Yellow("❌ Conexión rechazada.")

		case "b":
			color.Red("⛔ Usuario bloqueado. (Implementar lógica de bloqueo si se desea)")

		default:
			color.Red("⚠️ Respuesta inválida, no se acepta la conexión.")
		}

	case "answer":
		pc.SetRemoteDescription(*s.SDP)

	case "ice":
		if s.ICE != nil {
			pc.AddICECandidate(*s.ICE)
		}
	}
}

func saveChats() {
	file, err := os.Create("chat_history.json")
	if err != nil {
		color.Red("❌ No se pudo guardar historial: %v", err)
		return
	}
	defer file.Close()
	json.NewEncoder(file).Encode(chats)
}

func loadChats() {
	file, err := os.Open("chat_history.json")
	if err != nil {
		return // No existe, es normal al principio
	}
	defer file.Close()
	json.NewDecoder(file).Decode(&chats)
}

func main() {
	clearScreen()
	loadChats()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	banner := `
	╔═════════════════════════════════════════════════╗
	║            ChatP2P - Cliente Terminal           ║
	║          Desarrollado por Miguel en Go          ║
	╚═════════════════════════════════════════════════╝
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

		case "/clear":
			if len(parts) < 2 {
				color.Red("Uso: /clear <usuario>")
				break
			}
			user := strings.TrimSpace(parts[1])
			var found bool
			for k := range chats {
				if strings.EqualFold(k, user) {
					delete(chats, k)
					found = true
					break
				}
			}
			if !found {
				color.Yellow("📭 No hay mensajes con %s.", user)
				break
			}
			saveChats()
			color.Yellow("🧹 Mensajes con %s eliminados.", user)

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
			err = json.NewDecoder(bytes.NewReader(raw)).Decode(&base)
			if err != nil {
				log.Printf("❌ Error decodificando tipo de mensaje: %v", err)
				continue
			}
			log.Printf("➡️ Tipo de mensaje detectado: '%s'", base.Type)
			switch base.Type {
			case "user-list":
				var ul struct {
					Type  string   `json:"type"`
					Users []string `json:"users"`
				}
				json.NewDecoder(bytes.NewReader(raw)).Decode(&ul)
				activeUsers = ul.Users
				userListCh <- ul.Users

			case "text":
				var temp struct {
					From      string `json:"from"`
					To        string `json:"to"`
					Content   string `json:"content"`
					Type      string `json:"type"`
					Timestamp string `json:"timestamp"`
				}

				if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&temp); err != nil {
					color.Red("❌ Error parseando mensaje: %v", err)
					break
				}

				t, err := time.Parse(time.RFC3339, temp.Timestamp)
				if err != nil {
					t = time.Now()
				}

				m := Message{
					From:      temp.From,
					To:        temp.To,
					Content:   temp.Content,
					Type:      temp.Type,
					Timestamp: t,
				}

				key := strings.TrimSpace(m.From)
				if self != "" && m.From == self {
					key = strings.TrimSpace(m.To)
				}
				fmt.Printf("💾 Guardando mensaje bajo clave: %s\n", key)
				chats[key] = append(chats[key], m)
				saveChats()
				fmt.Printf("[%s]> %s\n", m.From, m.Content)
				fmt.Printf("📩 Mensaje de %s para %s: %s\n", m.From, m.To, m.Content)

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

		case "/chats":
			if len(parts) == 2 {
				user := strings.TrimSpace(parts[1])
				var found bool
				var msgs []Message
				for k, v := range chats {
					if strings.EqualFold(k, user) {
						msgs = v
						found = true
						break
					}
				}
				if !found || len(msgs) == 0 {
					color.Yellow("📭 No hay mensajes con %s.", user)
					break
				}
				color.Magenta("🗂️ Chat con %s:", user)
				for _, m := range msgs {
					fmt.Printf("  [%s] %s: %s\n", m.Timestamp.Format("15:04"), m.From, m.Content)
				}
			} else {
				if len(chats) == 0 {
					color.Yellow("📭 No hay mensajes almacenados aún.")
					break
				}
				color.Magenta("📚 Historial de chats:")
				fmt.Printf("🧪 Estado actual del historial:\n")
				for k, v := range chats {
				fmt.Printf("  Usuario: %s, Mensajes: %d\n", k, len(v))
				}
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
