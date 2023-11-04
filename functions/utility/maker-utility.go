package utility

import (
	"encoding/json"
	"errors"

	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/google/uuid"
)

var (
	ErrorCouldNotDynamoPutItem = "could not dynamo put item"
	ErrorCouldNotUnmarshalItem = "could not unmarshal maker request"
)

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

func BatchWriteToDynamoDB(roleCount int, makerRequests []MakerRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) ([]ReturnMakerRequest, error) {
	writeRequests := make([]*dynamodb.WriteRequest, roleCount)

	for i, request := range makerRequests {
		item, err := dynamodbattribute.MarshalMap(request)
		if err != nil {
			return nil, errors.New(ErrorCouldNotUnmarshalItem)
		}

		writeRequest := &dynamodb.WriteRequest{
			PutRequest: &dynamodb.PutRequest{
				Item: item,
			},
		}

		writeRequests[i] = writeRequest
	}

	input := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]*dynamodb.WriteRequest{
			tableName: writeRequests,
		},
	}
	_, err := dynaClient.BatchWriteItem(input)

	if err != nil {
		return nil, errors.New(ErrorCouldNotDynamoPutItem)
	}
	return FormatMakerRequest(makerRequests), nil
}

func FormatMakerRequest(makerRequests []MakerRequest) []ReturnMakerRequest {
	makerRequestsMap := make(map[string]ReturnMakerRequest)
	for _, request := range makerRequests {
		resRequest := makerRequestsMap[request.RequestUUID]
		if resRequest.RequestUUID == "" {
			makerRequestsMap[request.RequestUUID] = ReturnMakerRequest{
				RequestUUID:   request.RequestUUID,
				CheckerRole:   []string{request.CheckerRole},
				MakerUUID:     request.MakerUUID,
				CheckerUUID:   request.CheckerUUID,
				RequestStatus: request.RequestStatus,
				ResourceType:  request.ResourceType,
				RequestData:   request.RequestData,
			}
		} else {
			resRequest.CheckerRole = append(resRequest.CheckerRole, request.CheckerRole)
			makerRequestsMap[request.RequestUUID] = resRequest
		}
	}
	retRequests := make([]ReturnMakerRequest, 0, len(makerRequestsMap))
	for _, value := range makerRequestsMap {
		retRequests = append(retRequests, value)
	}

	return retRequests
}

func DeconstructPostMakerRequest(postMakerRequest NewMakerRequest) []MakerRequest {
	roleCount := len(postMakerRequest.CheckerRoles)
	makerRequests := make([]MakerRequest, roleCount)
	reqId := uuid.NewString()

	for i := 0; i < roleCount; i++ {
		var makerRequest MakerRequest

		makerRequest.RequestUUID = reqId
		makerRequest.RequestStatus = "pending"
		makerRequest.CheckerUUID = ""
		makerRequest.CheckerRole = postMakerRequest.CheckerRoles[i]
		makerRequest.MakerUUID = postMakerRequest.MakerUUID
		makerRequest.ResourceType = postMakerRequest.ResourceType
		makerRequest.RequestData = postMakerRequest.RequestData

		makerRequests[i] = makerRequest
	}
	return makerRequests
}
