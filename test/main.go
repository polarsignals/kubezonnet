package main

import (
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

func main() {
	mode := getEnv("MODE", "server")

	if mode == "server" {
		runServer()
	} else {
		runClient()
	}
}

func runServer() {
	port := getEnv("PORT", "8888")
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	log.Printf("Server listening on :%s", port)

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

	buf := make([]byte, 32*1024)
	totalReceived := uint64(0)
	lastLog := time.Now()

	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading: %v", err)
			}
			log.Printf("Connection closed. Total received: %d bytes (%.2f MB)", totalReceived, float64(totalReceived)/(1024*1024))
			return
		}

		totalReceived += uint64(n)

		if time.Since(lastLog) >= time.Second {
			log.Printf("Received: total=%d bytes (%.2f MB)", totalReceived, float64(totalReceived)/(1024*1024))
			lastLog = time.Now()
		}

		// Send tiny ACK
		conn.Write([]byte("ok"))
	}
}

func runClient() {
	serverAddr := getEnv("SERVER", "test-server:8888")
	chunkSizeMB := getEnvInt("CHUNK_SIZE_MB", 1)
	intervalMs := getEnvInt("INTERVAL_MS", 100)

	log.Printf("Client mode: server=%s, chunk=%dMB, interval=%dms", serverAddr, chunkSizeMB, intervalMs)

	time.Sleep(5 * time.Second) // Wait for server

	for {
		conn, err := net.Dial("tcp", serverAddr)
		if err != nil {
			log.Printf("Error connecting: %v, retrying in 5s...", err)
			time.Sleep(5 * time.Second)
			continue
		}

		log.Println("Connected to server")

		chunk := make([]byte, chunkSizeMB*1024*1024)
		for i := range chunk {
			chunk[i] = byte(i % 256)
		}

		totalSent := uint64(0)
		ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
		defer ticker.Stop()

		lastLog := time.Now()

		for range ticker.C {
			n, err := conn.Write(chunk)
			if err != nil {
				log.Printf("Error writing: %v", err)
				conn.Close()
				break
			}

			totalSent += uint64(n)

			// Read ACK
			ack := make([]byte, 2)
			conn.Read(ack)

			if time.Since(lastLog) >= time.Second {
				log.Printf("Sent: total=%d bytes (%.2f MB)", totalSent, float64(totalSent)/(1024*1024))
				lastLog = time.Now()
			}
		}
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}
