package net

import (
    "encoding/json"
    "fmt"
    "net"
    "time"
    "p2p-chat/core"
)

// StartClient intenta conectarse a otro nodo en la red
func StartClient(remoteAddr string, nodeName string) net.Conn {
    fmt.Printf("[CONNECTING] %s intentando conectar con %s...\n", nodeName, remoteAddr)

    conn, err := net.DialTimeout("tcp", remoteAddr, 5*time.Second)
    if err != nil {
        fmt.Printf("[ERROR] No se pudo conectar con %s: %v\n", remoteAddr, err)
        return nil
    }

    fmt.Printf("[CONNECTED] %s conectado exitosamente con %s\n", nodeName, remoteAddr)

    // Enviar mensaje inicial
    msg := core.Message{
        From:      nodeName,
        To:        "servidor",
        Text:      "¡Hola desde el cliente!",
        Timestamp: time.Now(),
    }

    if err := SendMessage(conn, msg); err != nil {
        fmt.Printf("[ERROR] No se pudo enviar mensaje inicial: %v\n", err)
    }

    return conn
}

// SendMessage serializa y envía un mensaje a través de la conexión
func SendMessage(conn net.Conn, msg core.Message) error {
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

// CreateMessage crea un nuevo mensaje con los parámetros dados
func CreateMessage(from, to, text string) core.Message {
    return core.Message{
        From:      from,
        To:        to,
        Text:      text,
        Timestamp: time.Now(),
    }
}
