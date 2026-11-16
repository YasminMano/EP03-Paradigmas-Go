package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
)

type client chan<- string // canal de mensagem

var (
	entering = make(chan client)
	leaving  = make(chan client)
	messages = make(chan string)
)

func broadcaster() {
	clients := make(map[client]bool) // todos os clientes conectados
	for {
		select {
		case msg := <-messages:
			// broadcast de mensagens. Envio para todos
			for cli := range clients {
				cli <- msg
			}
		case cli := <-entering:
			clients[cli] = true
		case cli := <-leaving:
			delete(clients, cli)
			close(cli)
		}
	}
}

func clientWriter(conn net.Conn, ch <-chan string) {
	for msg := range ch {
		fmt.Fprintln(conn, msg)
	}
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	ch := make(chan string)
	go clientWriter(conn, ch)

	input := bufio.NewScanner(conn)

	// A primeira mensagem deve identificar o usuário
	if !input.Scan() {
		close(ch)
		return
	}

	identificacao := input.Text()

	if !strings.HasPrefix(identificacao, "NICK ") {
		fmt.Fprintln(conn, "Erro: informe seu apelido primeiro.")
		close(ch)
		return
	}

	apelido := strings.TrimSpace(
		strings.TrimPrefix(identificacao, "NICK "),
	)

	if apelido == "" {
		fmt.Fprintln(conn, "Erro: apelido inválido.")
		close(ch)
		return
	}

	// Registra o cliente no broadcaster
	entering <- ch

	mensagemEntrada := "Usuário @" + apelido + " acabou de entrar"

	fmt.Println(mensagemEntrada)
	messages <- mensagemEntrada

	// Recebe as mensagens do usuário
	for input.Scan() {
		messages <- "@" + apelido + " disse: " + input.Text()
	}

	// Remove o usuário quando ele desconecta
	leaving <- ch

	mensagemSaida := "Usuário @" + apelido + " saiu"

	fmt.Println(mensagemSaida)
	messages <- mensagemSaida
}

func main() {
	fmt.Println("Iniciando servidor...")
	listener, err := net.Listen("tcp", "0.0.0.0:4444")
	fmt.Println("Portas abertas!")
	if err != nil {
		log.Fatal(err)
	}
	go broadcaster()
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		go handleConn(conn)
	}
}
