package types

import "encoding/json"

type DecisionBody struct {
	RequestId   string `json:"request_id"`
	CheckerRole string `json:"checker_role"`
	CheckerId   string `json:"checker_id"`
	Decision    string `json:"decision"`
}

type MakerRequest struct {
	RequestUUID   string          `json:"req_id"`
	CheckerRole   string          `json:"checker_role"`
	MakerUUID     string          `json:"maker_id"`
	CheckerUUID   string          `json:"checker_id"`
	RequestStatus string          `json:"request_status"`
	ResourceType  string          `json:"resource_type"`
	RequestData   json.RawMessage `json:"request_data"`
}

type ReturnMakerRequest struct {
	RequestUUID   string          `json:"req_id"`
	CheckerRole   []string        `json:"checker_role"`
	MakerUUID     string          `json:"maker_id"`
	CheckerUUID   string          `json:"checker_id"`
	RequestStatus string          `json:"request_status"`
	ResourceType  string          `json:"resource_type"`
	RequestData   json.RawMessage `json:"request_data"`
}

type NewMakerRequest struct {
	CheckerRoles []string        `json:"checker_roles"`
	MakerUUID    string          `json:"maker_id"`
	ResourceType string          `json:"resource_type"`
	RequestData  json.RawMessage `json:"request_data"`
}

type ReturnMakerData struct {
	Data    []ReturnMakerRequest `json:"data"`
	KeyReq  string               `json:"key_req"`
	KeyRole string               `json:"key_role"`
}
