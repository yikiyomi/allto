package repo
import(
	"allto/model"
	"gorm.io/gorm"
)
type OutboxRepo struct{
	db *gorm.DB
}
func NewOutboxRepo(db *gorm.DB) *OutboxRepo {return &OutboxRepo{db:db}}
func (r *OutboxRepo) WithTx(tx *gorm.DB) *OutboxRepo{return &OutboxRepo{db:tx}}

func (r *OutboxRepo) Create(o *model.Outbox) error{return r.db.Create(o).Error}
func (r *OutboxRepo) ListPending(limit int)([]model.Outbox,error){
	var msgs []model.Outbox
	err:=r.db.Order("id").Limit(limit).Find(&msgs).Error
	return msgs,err
}
func(r *OutboxRepo) Delete(id uint) error {return r.db.Delete(&model.Outbox{},id).Error}