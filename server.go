package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
)

type client struct {
	apelido   string
	mensagens chan string
}

type pedidoEntrada struct {
	usuario  *client
	resposta chan bool
}

type pedidoTrocaApelido struct {
	usuario     *client
	novoApelido string
	resposta    chan bool
}

var (
	entering      = make(chan pedidoEntrada)
	leaving       = make(chan *client)
	messages      = make(chan string)
	trocasApelido = make(chan pedidoTrocaApelido)
)

func broadcaster() {
	clients := make(map[*client]bool)
	apelidos := make(map[string]*client)

	for {
		select {
		case msg := <-messages:
			// Envia a mensagem para todos os clientes conectados
			for cli := range clients {
				cli.mensagens <- msg
			}

		case pedido := <-entering:
			cli := pedido.usuario

			if _, existe := apelidos[cli.apelido]; existe {
				pedido.resposta <- false
				continue
			}

			clients[cli] = true
			apelidos[cli.apelido] = cli

			pedido.resposta <- true

		case cli := <-leaving:
			delete(clients, cli)
			delete(apelidos, cli.apelido)
			close(cli.mensagens)

		case pedido := <-trocasApelido:
			cli := pedido.usuario
			novo := strings.TrimSpace(pedido.novoApelido)

			// Verifica se o novo apelido é válido
			if novo == "" {
				pedido.resposta <- false
				continue
			}

			// Impede que outro usuário utilize o mesmo apelido
			if existente, ocupado := apelidos[novo]; ocupado && existente != cli {
				pedido.resposta <- false
				continue
			}

			// Guarda o apelido antigo
			antigo := cli.apelido

			// Atualiza o mapa e o usuário
			delete(apelidos, antigo)
			cli.apelido = novo
			apelidos[novo] = cli

			pedido.resposta <- true
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

	// Cria e registra o cliente
	usuario := &client{
		apelido:   apelido,
		mensagens: ch,
	}

	resposta := make(chan bool)

	entering <- pedidoEntrada{
		usuario:  usuario,
		resposta: resposta,
	}

	if !<-resposta {
		fmt.Fprintln(conn, "Erro: apelido já está em uso.")
		close(ch)
		return
	}

	mensagemEntrada := "Usuário @" + apelido + " acabou de entrar"

	fmt.Println(mensagemEntrada)
	messages <- mensagemEntrada

	// Recebe as mensagens e os comandos do usuário
	for input.Scan() {
		texto := input.Text()

		if strings.HasPrefix(texto, `\changenick `) {
			novoApelido := strings.TrimSpace(
				strings.TrimPrefix(texto, `\changenick `),
			)

			resposta := make(chan bool)

			trocasApelido <- pedidoTrocaApelido{
				usuario:     usuario,
				novoApelido: novoApelido,
				resposta:    resposta,
			}

			if <-resposta {
				antigo := apelido
				apelido = novoApelido

				mensagem := "Usuário @" + antigo + " agora é @" + novoApelido

				fmt.Println(mensagem)
				messages <- mensagem
			} else {
				ch <- "Erro: apelido inválido ou já está em uso."
			}

			continue
		}

		messages <- "@" + apelido + " disse: " + texto
	}

	// Remove o usuário quando ele desconecta
	leaving <- usuario

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
