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
	bot       bool
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

type pedidoMensagemPrivada struct {
	remetente    *client
	destinatario string
	conteudo     string
	resposta     chan bool
}

var (
	entering          = make(chan pedidoEntrada)
	leaving           = make(chan *client)
	messages          = make(chan string)
	trocasApelido     = make(chan pedidoTrocaApelido)
	mensagensPrivadas = make(chan pedidoMensagemPrivada)
)

func broadcaster() {
	clients := make(map[*client]bool)
	apelidos := make(map[string]*client)

	// Envia sem deixar um cliente lento bloquear o servidor.
	enviar := func(cli *client, msg string) {
		select {
		case cli.mensagens <- msg:
		default:
			log.Printf("Fila de mensagens cheia para @%s", cli.apelido)
		}
	}

	for {
		select {
		case msg := <-messages:
			for cli := range clients {
				if !cli.bot {
					enviar(cli, msg)
				}
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

			if novo == "" {
				pedido.resposta <- false
				continue
			}

			if existente, ocupado := apelidos[novo]; ocupado && existente != cli {
				pedido.resposta <- false
				continue
			}

			antigo := cli.apelido
			delete(apelidos, antigo)
			cli.apelido = novo
			apelidos[novo] = cli

			pedido.resposta <- true

		case pedido := <-mensagensPrivadas:
			destino, existe := apelidos[pedido.destinatario]

			if !existe {
				pedido.resposta <- false
				continue
			}

			mensagem := "@" + pedido.remetente.apelido +
				" disse em privado: " + pedido.conteudo

			// Apenas o destinatário recebe a mensagem.
			enviar(destino, mensagem)

			// O servidor registra a mensagem privada.
			fmt.Println(mensagem)

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

	ch := make(chan string, 32)
	go clientWriter(conn, ch)

	input := bufio.NewScanner(conn)

	// A primeira mensagem deve identificar o usuário
	if !input.Scan() {
		close(ch)
		return
	}

	identificacao := input.Text()

	ehBot := strings.HasPrefix(identificacao, "BOT ")

	if !ehBot && !strings.HasPrefix(identificacao, "NICK ") {
		fmt.Fprintln(conn, "Erro: identificação inválida.")
		close(ch)
		return
	}

	var apelido string

	if ehBot {
		apelido = strings.TrimSpace(
			strings.TrimPrefix(identificacao, "BOT "),
		)
	} else {
		apelido = strings.TrimSpace(
			strings.TrimPrefix(identificacao, "NICK "),
		)
	}

	if apelido == "" {
		fmt.Fprintln(conn, "Erro: apelido inválido.")
		close(ch)
		return
	}

	// Cria e registra o cliente
	usuario := &client{
		apelido:   apelido,
		mensagens: ch,
		bot:       ehBot,
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

	tipo := "Usuário"
	if usuario.bot {
		tipo = "Bot"
	}

	mensagemEntrada := tipo + " @" + apelido + " acabou de entrar"

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

		// Comandos de mensagens públicas e privadas
		if texto == `\msg` || strings.HasPrefix(texto, `\msg `) {
			conteudo := strings.TrimSpace(
				strings.TrimPrefix(texto, `\msg`),
			)

			if conteudo == "" {
				ch <- "Erro: a mensagem não pode estar vazia."
				continue
			}

			// Mensagem privada: \msg @Leo Olá!
			if strings.HasPrefix(conteudo, "@") {
				partes := strings.SplitN(conteudo, " ", 2)

				if len(partes) < 2 || strings.TrimSpace(partes[1]) == "" {
					ch <- "Erro: informe o destinatário e a mensagem."
					continue
				}

				destinatario := strings.TrimPrefix(partes[0], "@")
				mensagem := strings.TrimSpace(partes[1])

				resposta := make(chan bool)

				mensagensPrivadas <- pedidoMensagemPrivada{
					remetente:    usuario,
					destinatario: destinatario,
					conteudo:     mensagem,
					resposta:     resposta,
				}

				if !<-resposta {
					ch <- "Erro: destinatário não encontrado."
				}

				continue
			}

			// Mensagem pública
			messages <- "@" + apelido + " disse: " + conteudo
			continue
		}

		// Comando não reconhecido
		ch <- "Erro: utilize \\msg para enviar mensagens."
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
