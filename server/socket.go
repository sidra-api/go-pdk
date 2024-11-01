package server

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

type Request struct {
	Method string
	Path string
	Headers map[string]string
	Body string
}

type Response struct {
	StatusCode int
	Headers map[string]string
	Body string
}

type Server struct {
	listener net.Listener
	sockPath string
	access func(Request) Response
}

func NewServer(name string, access func(Request) Response) *Server {
	return &Server{
		sockPath: "/tmp/" + name,
		access: access,
	}
}

func (s *Server) Start() error {
	// Remove existing socket file if it exists
	if err := os.RemoveAll(s.sockPath); err != nil {
		return err
	}

	var err error
	s.listener, err = net.Listen("unix", s.sockPath)
	if err != nil {
		return err
	}

	// Set appropriate permissions for the socket file
	if err := os.Chmod(s.sockPath, 0666); err != nil {
		return err
	}

	// Handle graceful shutdown
	go s.handleShutdown()

	log.Printf("Unix domain socket server listening on %s", s.sockPath)

	// Accept connections
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}

		// Handle each connection in a goroutine
			go s.handleConnection(conn, s.access)
	}
}

// handleConnection processes individual client connections
func (s *Server) handleConnection(conn net.Conn, access func(Request) Response) {
	defer conn.Close()

	buffer := make([]byte, 1024)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			log.Printf("Error reading from connection: %v", err)
			return
		}

		// Process the received data
		message := buffer[:n]
		log.Printf("Received message: %s", string(message))
		var request Request
		json.Unmarshal(message, &request)
		response := s.access(request)
		// Echo back to client
		responseMessage, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error marshalling response: %v", err)
			return
		}
		_, err = conn.Write(responseMessage)
		if err != nil {
			log.Printf("Error writing to connection: %v", err)
			return
		}
	}
}

// handleShutdown implements graceful shutdown
func (s *Server) handleShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutting down socket server...")
	
	if err := s.listener.Close(); err != nil {
		log.Printf("Error closing listener: %v", err)
	}
	
	// Clean up the socket file
	if err := os.RemoveAll(s.sockPath); err != nil {
		log.Printf("Error removing socket file: %v", err)
	}
}
