package net

import (
    "encoding/json"
    "fmt"
    "io"
    "net"
    "p2p-chat/core"
)

// StartServer inicia un servidor TCP en el puerto indicado
func StartServer(port int, nodeName string, peerList *core.PeerList) error {
    address := fmt.Sprintf("0.0.0.0:%d", port)

    listener, err := net.Listen("tcp", address)
    if err != nil {
        return fmt.Errorf("no se pudo iniciar el servidor: %v", err)
    }
    defer listener.Close()

    fmt.Printf("[LISTENING] %s escuchando en %s...\n", nodeName, address)

    for {
        conn, err := listener.Accept()
        if err != nil {
            return fmt.Errorf("fallo al aceptar conexión: %v", err)
        }

        go handleConnection(conn, peerList)
    }
}

// handleConnection gestiona una nueva conexión entrante
func handleConnection(conn net.Conn, peerList *core.PeerList) {
    defer conn.Close()
    remoteAddr := conn.RemoteAddr().String()
    fmt.Printf("[NEW CONNECTION] Nodo remoto conectado desde %s\n", remoteAddr)

    // Leer mensaje inicial para obtener el nombre del peer
    buffer := make([]byte, 1024)
    n, err := conn.Read(buffer)
    if err != nil {
        fmt.Printf("[ERROR] Al leer mensaje inicial desde %s: %v\n", remoteAddr, err)
        return
    }

    // Deserializar mensaje inicial
    var msg core.Message
    if err := json.Unmarshal(buffer[:n], &msg); err != nil {
        fmt.Printf("[ERROR] No se pudo decodificar mensaje inicial: %v\n", err)
        return
    }

    // Agregar el peer a la lista
    peerList.AddPeer(msg.From, conn)
    fmt.Printf("[INFO] Nuevo peer agregado: %s\n", msg.From)

    // Bucle infinito para leer mensajes continuamente
    for {
        // Leer datos entrantes
        buffer := make([]byte, 1024)
        n, err := conn.Read(buffer)
        
        if err == io.EOF {
            fmt.Printf("[INFO] Conexión cerrada por %s\n", msg.From)
            peerList.RemovePeer(msg.From)
            return
        }
        if err != nil {
            fmt.Printf("[ERROR] Al leer desde %s: %v\n", msg.From, err)
            peerList.RemovePeer(msg.From)
            return
        }

        // Deserializar mensaje
        var newMsg core.Message
        if err := json.Unmarshal(buffer[:n], &newMsg); err != nil {
            fmt.Printf("[ERROR] No se pudo decodificar JSON: %v\n", err)
            continue
        }

        fmt.Printf("[RECEIVED] %s → %s: %s\n", newMsg.From, newMsg.To, newMsg.Text)
    }
}