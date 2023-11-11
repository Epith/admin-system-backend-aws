package utility

import (
	"ascenda/types"
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

func BatchWriteToDynamoDB(roleCount int, makerRequests []types.MakerRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) ([]types.ReturnMakerRequest, error) {
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

func FormatMakerRequest(makerRequests []types.MakerRequest) []types.ReturnMakerRequest {
	makerRequestsMap := make(map[string]types.ReturnMakerRequest)
	for _, request := range makerRequests {
		resRequest := makerRequestsMap[request.RequestUUID]
		if resRequest.RequestUUID == "" {
			makerRequestsMap[request.RequestUUID] = types.ReturnMakerRequest{
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
	retRequests := make([]types.ReturnMakerRequest, 0, len(makerRequestsMap))
	for _, value := range makerRequestsMap {
		retRequests = append(retRequests, value)
	}

	return retRequests
}

func DeconstructPostMakerRequest(postMakerRequest types.NewMakerRequest) []types.MakerRequest {
	roleCount := len(postMakerRequest.CheckerRoles)
	makerRequests := make([]types.MakerRequest, roleCount)
	reqId := uuid.NewString()

	for i := 0; i < roleCount; i++ {
		var makerRequest types.MakerRequest

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
