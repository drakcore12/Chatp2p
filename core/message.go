package core

import "time"

// Message representa un mensaje enviado entre nodos
type Message struct {
    From      string    `json:"from"`
    To        string    `json:"to"`
    Text      string    `json:"text"`
    Timestamp time.Time `json:"timestamp"`
}