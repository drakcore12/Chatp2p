package net

import (
    "fmt"
    "net"
    "time"
    "p2p-chat/core"
)

// ListenForPeers listens for broadcast messages to discover peers
func ListenForPeers(peerList *core.PeerList) {
    addr, err := net.ResolveUDPAddr("udp", ":9999")
    if err != nil {
        fmt.Printf("[ERROR] Unable to resolve UDP address: %v\n", err)
        return
    }

    conn, err := net.ListenUDP("udp", addr)
    if err != nil {
        fmt.Printf("[ERROR] Unable to listen on UDP: %v\n", err)
        return
    }
    defer conn.Close()

    buffer := make([]byte, 1024)
    for {
        n, remoteAddr, err := conn.ReadFromUDP(buffer)
        if err != nil {
            fmt.Printf("[ERROR] Unable to read from UDP: %v\n", err)
            continue
        }

        message := string(buffer[:n])
        if len(message) > 9 && message[:9] == "DISCOVER:" {
            peerName := message[9:]
            fmt.Printf("[INFO] Discovered peer: %s from %s\n", peerName, remoteAddr)
            // Add the peer to the list if not already present
            if _, exists := peerList.GetPeer(peerName); !exists {
                peerList.AddPeer(peerName, nil) // Connection can be established later
            }
        }
    }
}

// BroadcastAddress is the address used for broadcasting
const BroadcastAddress = "255.255.255.255:9999"

// BroadcastPresence sends a broadcast message to announce the node's presence
func BroadcastPresence(nodeName string) {
    conn, err := net.Dial("udp", BroadcastAddress)
    if err != nil {
        fmt.Printf("[ERROR] Unable to connect for broadcast: %v\n", err)
        return
    }
    defer conn.Close()

    message := fmt.Sprintf("DISCOVER:%s", nodeName)
    for {
        _, err = conn.Write([]byte(message))
        if err != nil {
            fmt.Printf("[ERROR] Unable to send broadcast message: %v\n", err)
            return
        }
        fmt.Println("[INFO] Broadcast message sent")
        time.Sleep(5 * time.Second) // Broadcast every 5 seconds
    }
}

func DiscoverPeers(port int, nodeName string, peerList *core.PeerList) error {
    // Escanear puertos en la red local
    for p := 3000; p < 3010; p++ {
        if p == port {
            continue // Saltar nuestro propio puerto
        }

        go func(p int) {
            addr := fmt.Sprintf("127.0.0.1:%d", p)
            if conn, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
                fmt.Printf("[DISCOVERED] Peer encontrado en %s\n", addr)
                peerList.AddPeer(fmt.Sprintf("peer-%d", p), conn)
            }
        }(p)
    }
    return nil
}