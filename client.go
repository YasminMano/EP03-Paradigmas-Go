package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
)

func main() {
	var apelido string

	fmt.Print("Digite seu apelido: ")
	fmt.Scanln(&apelido)

	conn, err := net.Dial("tcp", "localhost:4444")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	fmt.Println("Connected!")
	fmt.Fprintln(conn, "NICK "+apelido)

	done := make(chan struct{})

	// Recebe mensagens do servidor
	go func() {
		io.Copy(os.Stdout, conn)
		close(done)
	}()

	// Lê comandos digitados pelo usuário
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		texto := scanner.Text()

		if texto == `\exit` {
			fmt.Println("Saindo do chat...")
			break
		}

		fmt.Fprintln(conn, texto)
	}

	// Avisa ao servidor que não enviaremos mais dados.
	if tcp, ok := conn.(*net.TCPConn); ok {
		tcp.CloseWrite()
	}

	// Aguarda o encerramento da leitura.
	<-done
}
