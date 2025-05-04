package common

import (
    "net"
    "github.com/pion/webrtc/v3"
    "time"
)

// Peer represents a connected node
type Peer struct {
    Name    string
    Conn    net.Conn
    Active  bool
    WebRTCConn *webrtc.PeerConnection
    WebRTCDataChannel *webrtc.DataChannel
}

// Message represents a communication message
type Message struct {
    From      string    `json:"from"`
    To        string    `json:"to"`
    Content   string    `json:"content"`
    Type      string    `json:"type"`
    Timestamp time.Time `json:"timestamp"`
}

// SignalingMessage represents WebRTC signaling data
type SignalingMessage struct {
    Type string
    SDP  string
    ICE  *webrtc.ICECandidateInit
}

// PeerManager defines the interface for peer management
type PeerManager interface {
    AddPeer(name string, conn net.Conn)
    RemovePeer(name string)
    GetPeer(name string) (*Peer, bool)
    GetActivePeers() []string
    SendMessage(to string, message string) error
}