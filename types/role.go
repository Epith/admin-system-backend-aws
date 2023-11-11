package types

type Role struct {
	Role   string              `json:"role"`
	Access map[string][]string `json:"access"`
}

type ReturnRoleData struct {
	Data []Role `json:"data"`
	Key  string `json:"key"`
}
