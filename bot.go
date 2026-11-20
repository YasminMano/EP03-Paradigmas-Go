package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
)

// Inverte o texto recebido.
func inverter(texto string) string {
	letras := []rune(texto)

	for i, j := 0, len(letras)-1; i < j; i, j = i+1, j-1 {
		letras[i], letras[j] = letras[j], letras[i]
	}

	return string(letras)
}

func main() {
	conn, err := net.Dial("tcp", "localhost:4444")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	apelido := "Inversor"

	// Identifica o programa como bot.
	fmt.Fprintln(conn, "BOT "+apelido)

	fmt.Println("Bot @" + apelido + " conectado!")

	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		linha := scanner.Text()

		partes := strings.SplitN(linha, " disse em privado: ", 2)
		if len(partes) != 2 {
			continue
		}

		remetente := strings.TrimPrefix(partes[0], "@")
		recebida := partes[1]
		resposta := inverter(recebida)

		fmt.Println("Mensagem de: @" + remetente)
		fmt.Println("Recebi: " + recebida)
		fmt.Println("Resposta: " + resposta)

		// Responde em privado.
		fmt.Fprintf(conn, "\\msg @%s %s\n", remetente, resposta)
	}

	if err := scanner.Err(); err != nil {
		log.Println("Erro na conexão:", err)
	}
}
