package worker
import (
	"allto/database"
	"allto/model"
	"allto/mq"
	"allto/repo"
	"allto/util"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)
//消费 notify.events：落库 Notification（DedupKey 幂等）→ Redis Publish → Ack//吃
func StartNotificationConsumer(){
	go func (){
		for{
			mqConn:=database.GetMQ()
			if mqConn==nil{
				time.Sleep(time.Second)
				continue
			}
			if err:=consumeLoop(mqConn);err!=nil{
				util.Logger.Error("notification断开，五秒后重试",zap.Error(err))
			}
			time.Sleep(5*time.Second)
		}
	}()
}
func consumeLoop(mqConn *mq.Rabbitmq)error{
	ch,err:=mqConn.NewChannel()
	if err!=nil{
		return err
	}
	defer ch.Close()
	
	if err:=mq.Declare(ch);err!=nil{
		return err
	}
	// 一次最多预取 10 条，处理完再要。不设的话 Broker 会把队列积压全推进客户端内存，
	// 且消费者一旦崩掉，这些未 Ack 的消息会全部重投。
	if err:=ch.Qos(10,0,false);err!=nil{
		return err
	}
	deliveries,err:=ch.Consume(mq.Queue,"",false,false,false,false,nil)
	if err!=nil{
		return err
	}
	notifRepo:=repo.NewNotificationRepo(database.GetDB())
	rdb:=database.GetRedis()

	for d:=range deliveries{
		var ev model.NotificationEvent
		if err:=json.Unmarshal(d.Body,&ev);err!=nil{
			// 坏的扔了
			util.Logger.Error("Notification:消息解析失败，消息丢失",zap.Error(err))
			_=d.Ack(false)
			continue
		}
		n:=model.Notification{
			RecipientID: ev.RecipientID,
			SenderID:    ev.SenderID,
			Type:        ev.Type,
			TargetID:    ev.TargetID,
			DedupKey:    ev.EventID,
		}
		if err:=notifRepo.Create(&n);err!=nil{
			if errors.Is(err,gorm.ErrDuplicatedKey){
				// 重复的不要
				util.Logger.Info("notification:重复消息，跳过",zap.Error(err))
				_ =d.Ack(false)
				continue
			}
			util.Logger.Error("notification:落库失败，重投",zap.Error(err))
			_=d.Nack(false,true)
			continue
		}
		if rdb!=nil{
			b,_:=json.Marshal(n)
			if err:=rdb.Publish(database.GetCtx(),
					"notify:"+strconv.Itoa(int(ev.RecipientID)),b).Err();err!=nil{
					util.Logger.Error("notification:Redis Publish 失败", zap.Error(err))
					}
		}
		_=d.Ack(false)
	}
	return nil
}