package main

import (
    "flag"
    "fmt"
    "log"
    netmod "p2p-chat/net"
    "p2p-chat/cli"
    "p2p-chat/core"
    "os"
    "os/signal"
    "sync"
    "syscall"
)

func main() {
    port := flag.Int("port", 3000, "Puerto para escuchar conexiones")
    name := flag.String("name", "Node1", "Nombre del nodo")
    flag.Parse()

    // Crear lista de peers
    peerList := core.NewPeerList()

    // Canal para señales de sistema
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    // WaitGroup para goroutines
    var wg sync.WaitGroup
    wg.Add(2) // Una para el servidor y otra para discovery

    // Iniciar servidor en una goroutine
    errChan := make(chan error, 2)
    go func() {
        defer wg.Done()
        if err := netmod.StartServer(*port, *name, peerList); err != nil {
            errChan <- fmt.Errorf("error en servidor: %v", err)
        }
    }()

    // Iniciar descubrimiento de peers en una goroutine
    go func() {
        defer wg.Done()
        if err := netmod.DiscoverPeers(*port, *name, peerList); err != nil {
            errChan <- fmt.Errorf("error en descubrimiento: %v", err)
        }
    }()

    // Goroutine para manejar errores
    go func() {
        for err := range errChan {
            log.Printf("[ERROR] %v\n", err)
        }
    }()

    // Iniciar interfaz de chat en la goroutine principal
    go cli.StartInputLoop(peerList, *name)

    // Esperar señal de terminación o error
    select {
    case <-sigChan:
        fmt.Println("\n[INFO] Cerrando el programa...")
        peerList.Close() // Cambiado de CloseAll a Close
        wg.Wait()
    case err := <-errChan:
        log.Fatalf("[FATAL] %v\n", err)
    }
}