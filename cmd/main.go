package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"p2p-chat/common"

	"github.com/gorilla/websocket"
)

func main() {
	fmt.Print("Por favor, ingresa tu nombre de usuario: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	username := scanner.Text()

	// Conectar al servidor WebSocket
	conn, _, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:8080/ws", nil)
	if err != nil {
		fmt.Println("No se pudo conectar al servidor:", err)
		return
	}
	defer conn.Close()

	// Enviar mensaje inicial con el nombre de usuario
	initMsg := map[string]string{"from": username}
	if err := conn.WriteJSON(initMsg); err != nil {
		fmt.Println("Error enviando nombre de usuario:", err)
		return
	}

	users := []string{}
	userListChan := make(chan []string)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	// Goroutine para recibir mensajes
	go func() {
		for {
			var msg map[string]interface{}
			if err := conn.ReadJSON(&msg); err != nil {
				fmt.Println("\n[ERROR] Conexión cerrada o error al leer mensajes:", err)
				os.Exit(0)
			}
			switch msg["type"] {
			case "user-list":
				// Actualizar lista de usuarios
				rawUsers, ok := msg["users"].([]interface{})
				if ok {
					users = []string{}
					for _, u := range rawUsers {
						if name, ok := u.(string); ok {
							users = append(users, name)
						}
					}
					userListChan <- users
				}
			case "text":
				from := msg["from"]
				content := msg["content"]
				fmt.Printf("\n[%v]: %v\n", from, content)
				fmt.Print("> ")
			}
		}
	}()

	fmt.Println("\nBienvenido al chat! Comandos disponibles:")
	fmt.Println("/help - Mostrar ayuda")
	fmt.Println("/exit - Salir del chat")
	fmt.Println("/list - Ver usuarios conectados")
	fmt.Println("/to <usuario> - Seleccionar destinatario")
	fmt.Println("@usuario <mensaje> - Mensaje privado")
	fmt.Println("@all <mensaje> - Mensaje a todos")
	fmt.Println("\nEscribe tu mensaje y presiona Enter para enviar")
	fmt.Print("> ")

	currentTo := ""
	reader := bufio.NewReader(os.Stdin)

	for {
		select {
		case <-interrupt:
			fmt.Println("\nSaliendo del chat...")
			return
		default:
			text, err := reader.ReadString('\n')
			if err != nil {
				fmt.Printf("[ERROR] Error leyendo entrada: %v\n", err)
				continue
			}
			text = strings.TrimSpace(text)

			switch {
			case text == "/help":
				fmt.Println("\nComandos disponibles:")
				fmt.Println("  /help           - Muestra esta ayuda")
				fmt.Println("  /list           - Lista usuarios conectados")
				fmt.Println("  /to <usuario>   - Selecciona destinatario para mensajes")
				fmt.Println("  /exit           - Salir del chat")
				fmt.Println("\nModos de envío:")
				fmt.Println("  @usuario msg    - Envía mensaje a un usuario")
				fmt.Println("  @all msg        - Envía mensaje a todos")
				fmt.Println("  msg             - Envía al destinatario actual")
				fmt.Print("> ")
				continue

			case text == "/exit":
				fmt.Println("Saliendo del chat...")
				return

			case text == "/list":
				// Solicitar lista de usuarios al servidor
				req := map[string]interface{}{"type": "list-users"}
				conn.WriteJSON(req)
				// Esperar actualización de usuarios
				select {
				case users = <-userListChan:
					fmt.Printf("[INFO] Usuarios conectados:\n- %s (tú)\n", username)
					for _, peer := range users {
						if peer != username {
							fmt.Printf("- %s\n", peer)
						}
					}
				case <-time.After(2 * time.Second):
					fmt.Println("[ERROR] No se pudo obtener la lista de usuarios.")
				}
				fmt.Print("> ")
				continue

			case strings.HasPrefix(text, "/to "):
				currentTo = strings.TrimSpace(strings.TrimPrefix(text, "/to "))
				fmt.Printf("[INFO] Destinatario cambiado a %s\n", currentTo)
				fmt.Print("> ")
				continue

			case strings.HasPrefix(text, "@all "):
				msg := strings.TrimPrefix(text, "@all ")
				// Enviar a todos los usuarios menos a sí mismo
				for _, peer := range users {
					if peer != username {
						sendMsg := common.Message{
							From:      username,
							To:        peer,
							Content:   msg,
							Type:      "text",
							Timestamp: time.Now(),
						}
						if err := conn.WriteJSON(sendMsg); err != nil {
							fmt.Printf("[ERROR] %v\n", err)
						}
					}
				}
				fmt.Print("> ")
				continue

			case strings.HasPrefix(text, "@"):
				parts := strings.SplitN(text, " ", 2)
				if len(parts) == 2 {
					usuario := strings.TrimPrefix(parts[0], "@")
					msg := parts[1]
					sendMsg := common.Message{
						From:      username,
						To:        usuario,
						Content:   msg,
						Type:      "text",
						Timestamp: time.Now(),
					}
					if err := conn.WriteJSON(sendMsg); err != nil {
						fmt.Printf("[ERROR] %v\n", err)
					}
				}
				fmt.Print("> ")
				continue

			case text != "":
				if currentTo == "" {
					fmt.Println("[INFO] Especifica un destinatario con @usuario o /to usuario")
				} else {
					sendMsg := common.Message{
						From:      username,
						To:        currentTo,
						Content:   text,
						Type:      "text",
						Timestamp: time.Now(),
					}
					if err := conn.WriteJSON(sendMsg); err != nil {
						fmt.Printf("[ERROR] %v\n", err)
					}
				}
				fmt.Print("> ")
			}
		}
	}
}
