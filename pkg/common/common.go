package common

import (
	"net"
)

// Message representa un mensaje entre peers o vía señalización
// Si no necesitas esta función getLocalIP, elimínala para evitar warnings.
//
// type Message struct {
// 	From      string    `json:"from"`
// 	To        string    `json:"to"`
// 	Content   string    `json:"content"`
// 	Type      string    `json:"type"`
// 	Timestamp time.Time `json:"timestamp"`
// }

// PeerManager define la interfaz común para manejo de peers
// (puedes quitar parámetros no usados si no aplican)
type PeerManager interface {
	AddPeer(name string, conn net.Conn)
	RemovePeer(name string)
	GetPeer(name string) (*Peer, bool)
	GetActivePeers() []string
	SendMessage(to string, msg Message) error
}
