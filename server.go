package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

type client struct {
	apelido    string
	mensagens  chan string
	bot        bool
	writerDone <-chan struct{}
}

// Valida apelidos utilizados no protocolo do chat.
func apelidoValido(apelido string) bool {
	if apelido == "" || strings.ContainsAny(apelido, " \t\r\n@") {
		return false
	}

	return true
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

type pedidoSaida struct {
	usuario  *client
	resposta chan struct{}
}

var (
	entering          = make(chan pedidoEntrada)
	leaving           = make(chan pedidoSaida)
	messages          = make(chan string)
	trocasApelido     = make(chan pedidoTrocaApelido)
	mensagensPrivadas = make(chan pedidoMensagemPrivada)
)

func broadcaster() {
	clients := make(map[*client]bool)
	apelidos := make(map[string]*client)

	// Tenta colocar a mensagem na fila do destinatário.
	// Retorna false quando a fila está cheia.
	enviar := func(cli *client, msg string) bool {
		select {
		case <-cli.writerDone:
			return false
		default:
		}

		select {
		case cli.mensagens <- msg:
			return true
		case <-cli.writerDone:
			return false
		default:
			log.Printf("Fila de mensagens cheia para @%s", cli.apelido)
			return false
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

		case pedido := <-leaving:
			cli := pedido.usuario

			if clients[cli] {
				delete(clients, cli)
				delete(apelidos, cli.apelido)
				close(cli.mensagens)
			}

			close(pedido.resposta)

		case pedido := <-trocasApelido:
			cli := pedido.usuario
			novo := strings.TrimSpace(pedido.novoApelido)

			if !apelidoValido(novo) {
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
			if !enviar(destino, mensagem) {
				pedido.resposta <- false
				continue
			}

			// O servidor registra a mensagem privada.
			fmt.Println(mensagem)

			pedido.resposta <- true
		}
	}
}

func clientWriter(conn net.Conn, ch <-chan string, done chan<- struct{}) {
	defer close(done)

	for msg := range ch {
		if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
			log.Printf("Erro ao definir prazo de escrita: %v", err)
			conn.Close()
			return
		}

		if _, err := fmt.Fprintln(conn, msg); err != nil {
			log.Printf("Erro ao enviar mensagem: %v", err)
			conn.Close()
			return
		}
	}
}

// Envia uma resposta ao cliente sem bloquear caso o escritor termine.
func responder(ch chan<- string, done <-chan struct{}, mensagem string) {
	select {
	case <-done:
		return
	default:
	}

	select {
	case ch <- mensagem:
	case <-done:
	}
}

func handleConn(conn net.Conn) {
	ch := make(chan string, 32)
	writerDone := make(chan struct{})

	go clientWriter(conn, ch, writerDone)

	defer func() {
		<-writerDone
		conn.Close()
	}()

	input := bufio.NewScanner(conn)

	// A primeira mensagem deve identificar o usuário
	if !input.Scan() {
		close(ch)
		return
	}

	identificacao := input.Text()

	ehBot := strings.HasPrefix(identificacao, "BOT ")

	if !ehBot && !strings.HasPrefix(identificacao, "NICK ") {
		responder(ch, writerDone, "Erro: identificação inválida.")
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

	if !apelidoValido(apelido) {
		responder(ch, writerDone, "Erro: apelido inválido.")
		close(ch)
		return
	}

	// Cria e registra o cliente
	usuario := &client{
		apelido:    apelido,
		mensagens:  ch,
		bot:        ehBot,
		writerDone: writerDone,
	}

	resposta := make(chan bool)

	entering <- pedidoEntrada{
		usuario:  usuario,
		resposta: resposta,
	}

	if !<-resposta {
		responder(ch, writerDone, "Erro: apelido já está em uso.")
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

		// Ignora linhas vazias.
		if strings.TrimSpace(texto) == "" {
			continue
		}

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
				responder(ch, writerDone, "Erro: apelido inválido ou já está em uso.")
			}

			continue
		}

		// Comandos de mensagens públicas e privadas
		if texto == `\msg` || strings.HasPrefix(texto, `\msg `) {
			conteudo := strings.TrimSpace(
				strings.TrimPrefix(texto, `\msg`),
			)

			if conteudo == "" {
				responder(ch, writerDone, "Erro: a mensagem não pode estar vazia.")
				continue
			}

			// Mensagem privada: \msg @Leo Olá!
			if strings.HasPrefix(conteudo, "@") {
				partes := strings.SplitN(conteudo, " ", 2)

				if len(partes) < 2 || strings.TrimSpace(partes[1]) == "" {
					responder(ch, writerDone, "Erro: informe o destinatário e a mensagem.")
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
					responder(ch, writerDone, "Erro: destinatário não encontrado ou indisponível.")
				}

				continue
			}

			// Mensagem pública
			messages <- "@" + apelido + " disse: " + conteudo
			continue
		}

		// Comando não reconhecido
		responder(ch, writerDone, "Erro: utilize \\msg para enviar mensagens.")
	}

	if err := input.Err(); err != nil {
		log.Printf("Erro ao ler dados de @%s: %v", apelido, err)
	}

	// Solicita a remoção e aguarda o fechamento da fila.
	saidaConfirmada := make(chan struct{})

	leaving <- pedidoSaida{
		usuario:  usuario,
		resposta: saidaConfirmada,
	}

	<-saidaConfirmada

	tipo = "Usuário"
	if usuario.bot {
		tipo = "Bot"
	}

	mensagemSaida := tipo + " @" + apelido + " saiu"

	fmt.Println(mensagemSaida)
	messages <- mensagemSaida
}

func main() {
	fmt.Println("Iniciando servidor...")
	listener, err := net.Listen("tcp", "0.0.0.0:4444")
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	fmt.Println("Portas abertas!")
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
