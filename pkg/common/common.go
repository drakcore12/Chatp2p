package common

import "time"

// Peer representa un nodo conectado
// Aunque CLI no lo use directamente, lo dejamos para futuras extensiones.
type Peer struct {
	Name   string
	Active bool
}

// Message representa un mensaje de chat o señalización
type Message struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Content   string    `json:"content"`
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
}
