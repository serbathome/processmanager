package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"time"
)

func main() {
	port := flag.Int("p", 3888, "Port to start the TCP server on")
	flag.Parse()

	address := fmt.Sprintf(":%d", *port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Error starting TCP server on port %d: %v", *port, err)
	}
	defer listener.Close()

	log.Printf("Server started on port %d", *port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	fmt.Printf("New connection to aefad from %s\n", conn.RemoteAddr().String())
	time.Sleep(10 * time.Second) // Wait for 10 seconds
}
