package inventory

import (
	"github.com/alphaqiu/ginrpc/mock/model"
	"github.com/alphaqiu/ginrpc/payload"
	"github.com/pkg/errors"
	"net/http"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("mock")

type Inventory struct{}

func (api *Inventory) NotUsed(a string) error {
	return nil
}

func (api *Inventory) name() {
	return
}

func (api *Inventory) Add(item model.InventoryModel) payload.Response {
	log.Debugf("Invoke Inventory Add Method -> Name:%s", item.Name)
	return &payload.DefaultResponse{Code: 400, Err: errors.New("mock error")}
}

func (api *Inventory) Remove(query model.InventoryQuery) payload.Response {
	log.Debugf("Invoke Inventory Remove Method[POST] -> Name: %s", query.Name)
	return &payload.DefaultResponse{Code: http.StatusOK}
}

func (api *Inventory) GetRemove(query model.InventoryQuery) (*model.InventoryModel, payload.Response) {
	log.Debugf("Invoke Inventory Remove Method[GET] -> Name: %s", query.Name)
	return &model.InventoryModel{Name: query.Name}, &payload.DefaultResponse{Code: 200}
}

func (api *Inventory) GetData(query model.InventoryQuery) (model.InventoryModel, payload.Response) {
	log.Debugf("Invoke Inventory Data Method[GET] -> Name: %s", query.Name)
	return model.InventoryModel{Name: query.Name}, &payload.DefaultResponse{Code: 201}
}

func (api *Inventory) GetEmpty() payload.Response {
	log.Debugf("Invoke Inventory Empty Method[GET] -> Empty")
	return &payload.DefaultResponse{Code: 200}
}

func (api *Inventory) OptionsEmpty() payload.Response {
	log.Debugf("Invoke Inventory Empty Method[OPIONS] -> Empty")
	return &payload.DefaultResponse{Code: 200}
}

func (api *Inventory) Query(query model.InventoryQuery, item *model.InventoryModel) payload.Response {
	log.Debugf("Invoke Inventory Query Method[POST] -> query-name: %s, body-name: %s", query.Name, item.Name)
	return &payload.DefaultResponse{Code: 200}
}

func (api *Inventory) Revert(item *model.InventoryModel, query model.InventoryQuery) payload.Response {
	log.Debugf("Invoke Inventory Query Method[POST] -> query-name: %s, body-name: %s", query.Name, item.Name)
	return &payload.DefaultResponse{Code: 200}
}

func (api *Inventory) Header(item *model.InventoryModel, query model.InventoryQuery, header http.Header) payload.Response {
	xLab := header.Get("x-lab")
	log.Debugf("Invoke Inventory Query Method[POST] -> query-name: %s, body-name: %s, x-lab header: %s", query.Name, item.Name, xLab)
	return &payload.DefaultResponse{Code: 200}
}

func (api *Inventory) List(item *model.InventoryModel, query model.InventoryQuery) ([]*model.InventoryModel, payload.Response) {
	log.Debugf("Invoke Inventory Query Method[POST] -> query-name: %s, body-name: %s, x-lab header: %s", query.Name, item.Name)

	return []*model.InventoryModel{&model.InventoryModel{Name: "alpha"}}, &payload.DefaultResponse{Code: 200}
}
