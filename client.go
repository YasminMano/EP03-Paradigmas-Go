package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
)

func mustCopy(dst io.Writer, src io.Reader) {
	if _, err := io.Copy(dst, src); err != nil {
		log.Fatal(err)
	}
}

func main() {
	// Solicita o apelido antes de conectar ao servidor
	var apelido string

	fmt.Print("Digite seu apelido: ")
	fmt.Scanln(&apelido)

	// Estabelece conexão TCP
	conn, err := net.Dial("tcp", "localhost:4444")

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Connected!")

	// Envia a identificação ao servidor
	fmt.Fprintln(conn, "NICK "+apelido)

	done := make(chan struct{})

	go func() {
		io.Copy(os.Stdout, conn)
		log.Println("done")
		done <- struct{}{} // sinaliza para a gorrotina principal
	}()
	mustCopy(conn, os.Stdin)
	conn.Close()
	<-done // espera a gorrotina terminar
}
