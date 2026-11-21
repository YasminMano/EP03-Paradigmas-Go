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

	if _, err := fmt.Scanln(&apelido); err != nil {
		log.Fatalf("Erro ao ler apelido: %v", err)
	}

	if apelido == "" {
		log.Fatal("Erro: o apelido não pode estar vazio.")
	}

	conn, err := net.Dial("tcp", "localhost:4444")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	fmt.Println("Connected!")
	if _, err := fmt.Fprintln(conn, "NICK "+apelido); err != nil {
		log.Fatalf("Erro ao enviar apelido: %v", err)
	}

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

		if _, err := fmt.Fprintln(conn, texto); err != nil {
			log.Printf("Erro ao enviar comando: %v", err)
			break
		}
	}

	// Verifica se ocorreu algum erro na leitura do teclado.
	if err := scanner.Err(); err != nil {
		log.Printf("Erro ao ler teclado: %v", err)
	}

	// Avisa ao servidor que não enviaremos mais dados.
	if tcp, ok := conn.(*net.TCPConn); ok {
		tcp.CloseWrite()
	}

	// Aguarda o encerramento da leitura.
	<-done
}
