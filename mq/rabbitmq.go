package mq
import (
	"fmt"
	"allto/config"
	amqp "github.com/rabbitmq/amqp091-go"
)
// 拓扑常量：一个 topic exchange + 一个队列 + 一个通配绑定
const(
	Exchange="notify.events"
	Queue="notify.events"
	BindingKey="notify.*"
)
type Rabbitmq struct{
	Conn *amqp.Connection
}
// new连接
func New(cfg config.RabbitMQConfig)(*Rabbitmq,error){
	url:=fmt.Sprintf("amqp://%s:%s@%s:%d/",cfg.Username,cfg.Password,cfg.Host,cfg.Port)
	conn,err:=amqp.Dial(url)
	if err!=nil{
		return nil,err
	}
	return &Rabbitmq{Conn: conn},nil
}
// close
func(r *Rabbitmq) Close()error{
	return r.Conn.Close()
}
// Declare 声明 exchange + queue + binding（幂等，可重复调用）
func Declare(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(Exchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(Queue, true, false, false, false, nil); err != nil {
		return err
	}
	return ch.QueueBind(Queue, BindingKey, Exchange, false, nil)
}

// Publish 发布通知事件，routingKey = "notify.<type>"
func Publish(ch *amqp.Channel, routingKey string, payload []byte) error {
	return ch.Publish(Exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         payload,
	})
}
// NewChannel 新建一个 Channel
func (r *Rabbitmq) NewChannel() (*amqp.Channel, error) {
	return r.Conn.Channel()
}