package core

import (
    "net"
    "sync"
)

// Peer representa un nodo conectado
type Peer struct {
    Name    string
    Conn    net.Conn
    Active  bool
}

// PeerList maneja la lista de peers conectados
type PeerList struct {
    peers map[string]*Peer
    mutex sync.RWMutex
}

// NewPeerList crea una nueva lista de peers
func NewPeerList() *PeerList {
    return &PeerList{
        peers: make(map[string]*Peer),
    }
}

// AddPeer agrega un nuevo peer a la lista
func (pl *PeerList) AddPeer(name string, conn net.Conn) {
    pl.mutex.Lock()
    defer pl.mutex.Unlock()
    pl.peers[name] = &Peer{
        Name:   name,
        Conn:   conn,
        Active: true,
    }
}

// RemovePeer elimina un peer de la lista
func (pl *PeerList) RemovePeer(name string) {
    pl.mutex.Lock()
    defer pl.mutex.Unlock()
    if peer, exists := pl.peers[name]; exists {
        peer.Active = false
        peer.Conn.Close()
        delete(pl.peers, name)
    }
}

// GetPeer obtiene un peer por su nombre
func (pl *PeerList) GetPeer(name string) (*Peer, bool) {
    pl.mutex.RLock()
    defer pl.mutex.RUnlock()
    peer, exists := pl.peers[name]
    return peer, exists
}

// GetActivePeers retorna la lista de nombres de peers activos
func (pl *PeerList) GetActivePeers() []string {
    pl.mutex.RLock()
    defer pl.mutex.RUnlock()
    var names []string
    for name, peer := range pl.peers {
        if peer.Active {
            names = append(names, name)
        }
    }
    return names
}

// BroadcastMessage envía un mensaje a todos los peers activos
func (pl *PeerList) BroadcastMessage(data []byte) {
    pl.mutex.RLock()
    defer pl.mutex.RUnlock()
    for _, peer := range pl.peers {
        if peer.Active {
            peer.Conn.Write(data)
        }
    }
}

// Close cierra todas las conexiones de peers
func (pl *PeerList) Close() {
    pl.mutex.Lock()
    defer pl.mutex.Unlock()
    for _, peer := range pl.peers {
        if peer.Active {
            peer.Conn.Close()
            peer.Active = false
        }
    }
}