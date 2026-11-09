package worker
import (
	"time"

	"allto/database"
	"allto/mq"
	"allto/repo"
	"allto/util"

	"go.uber.org/zap"
)
//扫outbox表扫到就发mq，发完删，失败了下轮重投‘
func StartOutboxPoller(){
	go func(){
		db:=database.GetDB()
		outboxRepo:=repo.NewOutboxRepo(db)
		for{
			mqConn:=database.GetMQ()
			if mqConn==nil{
				time.Sleep(time.Second)
				continue
			}
			pollOnce(mqConn,outboxRepo)
			time.Sleep(time.Second)
		}
	}()
}
// pollOnce 处理一轮：取一批 pending，逐条投递，成功删行
func pollOnce(mqConn *mq.Rabbitmq,outboxRepo *repo.OutboxRepo){
	ch,err:=mqConn.NewChannel()
	if err!=nil{
		util.Logger.Error("outbox:创建channel失败",zap.Error(err))
		return
	}
	defer ch.Close()
	// 声明拓扑
	if err:=mq.Declare(ch);err!=nil{
		util.Logger.Error("outbox:声明mq拓扑失败",zap.Error(err))
		return
	}
	msgs,err:=outboxRepo.ListPending(100)
	if err!=nil{
		util.Logger.Error("outbox:查询带投递消息失败",zap.Error(err))
		return
	}
	for _,msg:=range msgs{
		routingKey:="notify."+msg.EventType
		if err:=mq.Publish(ch,routingKey,[]byte(msg.Payload));err!=nil{
			// 投递失败，重投
			util.Logger.Error("outbox:投递失败",zap.Uint("id",msg.ID),zap.String("routing_key",routingKey),zap.Error(err))
			continue
		}
		if err:=outboxRepo.Delete(msg.ID);err!=nil{
			// 删除失败会导致消息重复发出
			util.Logger.Error("outbox:删除已投递消息失败",zap.Uint("id",msg.ID),zap.Error(err))
		}
	}
}