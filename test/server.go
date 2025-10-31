package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

func main() {
	listener, err := net.Listen("tcp", ":8888")
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	log.Println("Server listening on :8888")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error accepting:", err)
			continue
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	log.Printf("New connection from %s", conn.RemoteAddr())

	buf := make([]byte, 32*1024) // 32KB buffer
	totalReceived := uint64(0)
	lastLog := time.Now()

	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading: %v", err)
			}
			log.Printf("Connection closed. Total received: %d bytes", totalReceived)
			return
		}

		totalReceived += uint64(n)

		// Log every second
		if time.Since(lastLog) >= time.Second {
			log.Printf("Received %d bytes (total: %d)", n, totalReceived)
			lastLog = time.Now()
		}

		// Send tiny ACK response (just "ok")
		conn.Write([]byte("ok"))
	}
}
