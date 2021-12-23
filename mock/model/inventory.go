package model

type InventoryModel struct {
	Name string `json:"name"`
}

type InventoryQuery struct {
	Name string `form:"name"`
}
