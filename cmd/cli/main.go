package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"miniproyectoGO/pkg/common"

	"github.com/gorilla/websocket"
)

func main() {
	// Captura Ctrl+C
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	// Prompt de registro/login
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("¿Registrar (r) o Login (l)? ")
	scanner.Scan()
	choice := strings.TrimSpace(scanner.Text())

	fmt.Print("Usuario: ")
	scanner.Scan()
	username := strings.TrimSpace(scanner.Text())

	fmt.Print("Contraseña: ")
	scanner.Scan()
	password := strings.TrimSpace(scanner.Text())

	// Conexión WebSocket
	ws, _, err := websocket.DefaultDialer.Dial("ws://localhost:8080/ws", nil)
	if err != nil {
		fmt.Println("No se pudo conectar al servidor:", err)
		return
	}
	defer ws.Close()

	// Registro opcional
	if choice == "r" {
		ws.WriteJSON(map[string]string{"type": "register", "username": username, "password": password})
		var resp map[string]interface{}
		ws.ReadJSON(&resp)
		if resp["type"] != "register-success" {
			fmt.Println("Error registrando:", resp["message"])
			return
		}
		fmt.Println("Registro exitoso")
	}

	// Login
	ws.WriteJSON(map[string]string{"type": "login", "username": username, "password": password})
	var loginResp map[string]interface{}
	ws.ReadJSON(&loginResp)
	if loginResp["type"] != "login-success" {
		fmt.Println("Login fallido.")
		return
	}
	fmt.Println("Login exitoso como", username)

	// Lector de WS
	userListChan := make(chan []string)
	go func() {
		for {
			var msg map[string]interface{}
			if err := ws.ReadJSON(&msg); err != nil {
				fmt.Println("WebSocket cerrado:", err)
				os.Exit(1)
			}
			switch msg["type"] {
			case "user-list":
				raw := msg["users"].([]interface{})
				users := make([]string, len(raw))
				for i, u := range raw {
					users[i] = u.(string)
				}
				userListChan <- users
			case "text":
				fmt.Printf("\n[%s]: %s\n", msg["from"], msg["content"])
			}
		}
	}()

	// Loop CLI
	currentTo := ""
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)

		switch {
		case text == "/list":
			ws.WriteJSON(map[string]string{"type": "list-users"})
			users := <-userListChan
			fmt.Println("Usuarios online:")
			for _, u := range users {
				fmt.Println(" -", u)
			}

		case strings.HasPrefix(text, "/to "):
			currentTo = strings.TrimPrefix(text, "/to ")
			fmt.Println("Destinatario:", currentTo)

		case strings.HasPrefix(text, "@"):
			parts := strings.SplitN(text, " ", 2)
			if len(parts) == 2 {
				user := strings.TrimPrefix(parts[0], "@")
				msg := parts[1]
				ws.WriteJSON(common.Message{From: username, To: user, Content: msg, Type: "text", Timestamp: time.Now()})
			}

		case text != "":
			if currentTo == "" {
				fmt.Println("Usa /to <user> o @user antes de escribir.")
			} else {
				ws.WriteJSON(common.Message{From: username, To: currentTo, Content: text, Type: "text", Timestamp: time.Now()})
			}
		}
	}
}
