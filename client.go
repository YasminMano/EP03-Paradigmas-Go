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
	comandos := make(chan string)

	// Recebe mensagens do servidor.
	go func() {
		if _, err := io.Copy(os.Stdout, conn); err != nil {
			log.Printf("Erro ao receber mensagens: %v", err)
		}
		close(done)
	}()

	// Lê o teclado sem bloquear a rotina principal.
	go func() {
		defer close(comandos)

		scanner := bufio.NewScanner(os.Stdin)

		for scanner.Scan() {
			comandos <- scanner.Text()
		}

		if err := scanner.Err(); err != nil {
			log.Printf("Erro ao ler teclado: %v", err)
		}
	}()

	// Aguarda comandos do teclado ou encerramento do servidor.
	for {
		select {
		case <-done:
			fmt.Println("Conexão com o servidor encerrada.")
			return

		case texto, aberto := <-comandos:
			if !aberto || texto == `\exit` {
				if texto == `\exit` {
					fmt.Println("Saindo do chat...")
				}

				if tcp, ok := conn.(*net.TCPConn); ok {
					if err := tcp.CloseWrite(); err != nil {
						log.Printf("Erro ao encerrar envio: %v", err)
					}
				}

				<-done
				return
			}

			if _, err := fmt.Fprintln(conn, texto); err != nil {
				log.Printf("Erro ao enviar comando: %v", err)
				return
			}
		}
	}
}
