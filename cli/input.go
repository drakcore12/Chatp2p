package cli

import (
    "bufio"
    "encoding/json"
    "fmt"
    "net"
    "os"
    "strings"
    "time"

    "p2p-chat/core"
)

const (
    cmdHelp = "/help"
    cmdList = "/list"
    cmdExit = "/exit"
    cmdTo   = "/to"
)

// ShowHelp muestra la ayuda del chat
func ShowHelp() {
    fmt.Println("\nComandos disponibles:")
    fmt.Println("  /help           - Muestra esta ayuda")
    fmt.Println("  /list           - Lista usuarios conectados")
    fmt.Println("  /to <usuario>   - Selecciona destinatario para mensajes")
    fmt.Println("  /exit           - Salir del chat")
    fmt.Println("\nModos de envío:")
    fmt.Println("  @usuario msg    - Envía mensaje a un usuario")
    fmt.Println("  @all msg        - Envía mensaje a todos")
    fmt.Println("  msg             - Envía al destinatario actual")
}

// SendMessage envía un mensaje a través de la conexión
func SendMessage(conn net.Conn, from, to, text string) error {
    msg := core.Message{
        From:      from,
        To:        to,
        Text:      text,
        Timestamp: time.Now(),
    }

    data, err := json.Marshal(msg)
    if err != nil {
        return fmt.Errorf("error al serializar mensaje: %v", err)
    }
    
    _, err = conn.Write(data)
    if err != nil {
        return fmt.Errorf("error al enviar mensaje: %v", err)
    }
    
    return nil
}

// StartInputLoop lee mensajes desde teclado y los envía al peer
func StartInputLoop(peerList *core.PeerList, localName string) {
    reader := bufio.NewReader(os.Stdin)
    currentTo := ""

    ShowHelp()

    for {
        if currentTo == "" {
            fmt.Print("> ")
        } else {
            fmt.Printf("[To: %s]> ", currentTo)
        }
        
        text, _ := reader.ReadString('\n')
        text = strings.TrimSpace(text)

        switch {
        case text == cmdHelp:
            ShowHelp()
            continue

        case text == cmdList:
            peers := peerList.GetActivePeers()
            fmt.Printf("[INFO] Usuarios conectados:\n- %s (tú)\n", localName)
            for _, peer := range peers {
                fmt.Printf("- %s\n", peer)
            }
            continue

        case text != "":
            if currentTo == "" {
                fmt.Println("[INFO] Especifica un destinatario con @usuario o /to usuario")
            } else {
                if peer, exists := peerList.GetPeer(currentTo); exists {
                    if err := SendMessage(peer.Conn, localName, currentTo, text); err != nil {
                        fmt.Printf("[ERROR] %v\n", err)
                    }
                } else {
                    fmt.Printf("[ERROR] El usuario %s no está conectado\n", currentTo)
                }
            }
        }
    }
}