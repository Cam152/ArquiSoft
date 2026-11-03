// Package publisher es el único lugar que habla con agr-broker-as (conector c8).
package publisher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"

	"vote-service/internal/model"
)

var (
	// ErrNacked: el broker recibió el mensaje pero no lo aceptó.
	ErrNacked = errors.New("el broker rechazó el mensaje (nack)")
	// ErrUnroutable: el exchange no tiene colas enlazadas y el mensaje volvió.
	ErrUnroutable = errors.New("el evento no llegó a ninguna cola")
)

// AMQP publica VoteRegistered en el exchange agora.votes con confirmación del
// broker. Abre la conexión la primera vez que la necesita y la vuelve a abrir si
// se cae, así el servicio arranca aunque RabbitMQ todavía no esté listo.
type AMQP struct {
	url      string
	exchange string

	mu      sync.Mutex
	conn    *amqp.Connection
	ch      *amqp.Channel
	returns chan amqp.Return
}

func NewAMQP(url, exchange string) *AMQP {
	return &AMQP{url: url, exchange: exchange}
}

// Encode arma el mensaje AMQP de un evento.
func Encode(event model.VoteRegistered) (amqp.Publishing, error) {
	body, err := json.Marshal(event)
	if err != nil {
		return amqp.Publishing{}, err
	}
	return amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent, // sobrevive a un reinicio del broker
		MessageId:    event.EventID,
		Type:         event.Type,
		// Sin Timestamp: la hora exacta de publicación rompería el redondeo de la
		// hora del voto.
		Body: body,
	}, nil
}

// Publish entrega el evento y espera la confirmación (ack) del broker. Si algo
// falla, cierra la conexión para que el siguiente intento abra una nueva.
func (p *AMQP) Publish(ctx context.Context, event model.VoteRegistered) error {
	msg, err := Encode(event)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.connect(); err != nil {
		return err
	}
	drain(p.returns)
	// mandatory = true: si el exchange no tiene colas enlazadas, el broker
	// devuelve el mensaje (basic.return) en lugar de descartarlo en silencio.
	confirm, err := p.ch.PublishWithDeferredConfirmWithContext(ctx, p.exchange, "", true, false, msg)
	if err != nil {
		p.reset()
		return fmt.Errorf("no se pudo publicar: %w", err)
	}
	acked, err := confirm.WaitContext(ctx)
	if err != nil {
		p.reset()
		return fmt.Errorf("sin confirmación del broker: %w", err)
	}
	if !acked {
		return ErrNacked
	}
	// RabbitMQ envía el basic.return antes del ack del mismo mensaje, y las
	// publicaciones van una a la vez (mu), así que si volvió ya está en el canal.
	for _, r := range drain(p.returns) {
		if r.MessageId == event.EventID {
			return ErrUnroutable
		}
	}
	return nil
}

// drain vacía el canal de mensajes devueltos sin bloquear.
func drain(returns chan amqp.Return) []amqp.Return {
	var out []amqp.Return
	for {
		select {
		case r := <-returns:
			out = append(out, r)
		default:
			return out
		}
	}
}

// connect abre la conexión y un canal en modo confirmación. Llamar con mu tomado.
func (p *AMQP) connect() error {
	if p.conn != nil && !p.conn.IsClosed() && p.ch != nil && !p.ch.IsClosed() {
		return nil
	}
	p.reset()

	conn, err := amqp.DialConfig(p.url, amqp.Config{
		Properties: amqp.Table{"connection_name": "agr-vote-lo"},
	})
	if err != nil {
		return fmt.Errorf("no se pudo conectar a RabbitMQ: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return err
	}
	// La topología la declara agr-broker-as (definitions.json). Aquí solo se
	// comprueba que el exchange exista, sin cambiarlo.
	if err := ch.ExchangeDeclarePassive(p.exchange, amqp.ExchangeFanout, true, false, false, false, nil); err != nil {
		conn.Close()
		return fmt.Errorf("el exchange %q no existe en el broker: %w", p.exchange, err)
	}
	if err := ch.Confirm(false); err != nil {
		conn.Close()
		return err
	}
	p.returns = ch.NotifyReturn(make(chan amqp.Return, 16))
	p.conn, p.ch = conn, ch
	return nil
}

// reset cierra la conexión actual. Llamar con mu tomado.
func (p *AMQP) reset() {
	if p.conn != nil {
		p.conn.Close()
	}
	p.conn, p.ch = nil, nil
}

// Connected informa si hay una conexión abierta (lo muestra el health check).
func (p *AMQP) Connected() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.conn != nil && !p.conn.IsClosed()
}

func (p *AMQP) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reset()
}
