package inventory

import (
	"context"
	"fmt"
	"github.com/alphaqiu/ginrpc/mock/model"
	"github.com/pkg/errors"
	"net/http"
	"time"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("mock")

type Response interface {
	Code() int
	Message() string
	Header() http.Header
	Data() interface{}
}

type success struct{}

func (s *success) Code() int       { return 0 }
func (s *success) Message() string { return "" }
func (s *success) Error() string   { return "" }

func Success() *success {
	return &success{}
}

type failure struct {
	error
	msg  string
	code int
}

func (s *failure) Code() int       { return s.code }
func (s *failure) Message() string { return s.msg }
func (s *failure) Error() string   { return s.error.Error() }

func Failure(code int, msg string, err error) *failure {
	return &failure{error: err, msg: msg, code: code}
}

type Inventory struct{}

func (api *Inventory) Version() string {
	return "v1"
}

func (api *Inventory) NotUsed(a string) error {
	return nil
}

func (api *Inventory) name() {
	return
}

func (api *Inventory) Add(ctx context.Context, item model.InventoryModel) error {
	log.Debugf("Invoke Inventory Add Method -> Name:%s", item.Name)
	return Failure(400, "testing mock error", errors.New("mock error"))
}

func (api *Inventory) Remove(ctx context.Context, query model.InventoryQuery) error {
	log.Debugf("Invoke Inventory Remove Method[POST] -> Name: %s", query.Name)
	return Success()
}

func (api *Inventory) GetRemove(ctx context.Context, query model.InventoryQuery) (*model.InventoryModel, error) {
	log.Debugf("Invoke Inventory Remove Method[GET] -> Name: %s", query.Name)
	return &model.InventoryModel{Name: query.Name}, Success()
}

func (api *Inventory) GetData(ctx context.Context, query model.InventoryQuery) (model.InventoryModel, error) {
	log.Debugf("Invoke Inventory Data Method[GET] -> Name: %s", query.Name)
	return model.InventoryModel{Name: query.Name}, Success()
}

func (api *Inventory) GetEmpty(ctx context.Context) error {
	fmt.Println("GetEmpty in")
	time.Sleep(time.Second)
	fmt.Println("GetEmpty out")
	log.Debugf("Invoke Inventory Empty Method[GET] -> Empty")
	return nil
}

func (api *Inventory) OptionsEmpty(ctx context.Context) error {
	log.Debugf("Invoke Inventory Empty Method[OPIONS] -> Empty")
	return Success()
}

func (api *Inventory) Query(ctx context.Context, query model.InventoryQuery, item *model.InventoryModel) error {
	log.Debugf("Invoke Inventory Query Method[POST] -> query-name: %s, body-name: %s", query.Name, item.Name)
	return Success()
}

func (api *Inventory) Revert(ctx context.Context, item *model.InventoryModel, query model.InventoryQuery) error {
	log.Debugf("Invoke Inventory Query Method[POST] -> query-name: %s, body-name: %s", query.Name, item.Name)
	return Success()
}

func (api *Inventory) Header(ctx context.Context, item *model.InventoryModel, query model.InventoryQuery, header http.Header) error {
	xLab := header.Get("x-lab")
	log.Debugf("Invoke Inventory Query Method[POST] -> query-name: %s, body-name: %s, x-lab header: %s", query.Name, item.Name, xLab)
	return Success()
}

func (api *Inventory) List(ctx context.Context, item *model.InventoryModel, query model.InventoryQuery) ([]*model.InventoryModel, error) {
	log.Debugf("Invoke Inventory Query Method[POST] -> query-name: %s, body-name: %s", query.Name, item.Name)

	return []*model.InventoryModel{{Name: "alpha"}}, Success()
}
