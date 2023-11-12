package main

import (
	"ascenda/types"
	"ascenda/utility"
	"encoding/json"
	"errors"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/aws/aws-sdk-go/service/ses"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//getting variables
	region := os.Getenv("AWS_REGION")

	//setting up dynamo session
	awsSession, err := session.NewSession(&aws.Config{
		Region: aws.String(region)})

	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error setting up AWS session"),
		}, nil
	}

	dynaClient := dynamodb.New(awsSession)

	// Get the parameter value
	paramUser := "USER_TABLE"
	outputUser, err := utility.GetParameterValue(awsSession, paramUser)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error getting user table parameter store"),
		}, nil
	}
	USER_TABLE := *outputUser.Parameter.Value

	paramPoints := "POINTS_TABLE"
	outputPoints, err := utility.GetParameterValue(awsSession, paramPoints)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error getting points table parameter store"),
		}, nil
	}
	POINTS_TABLE := *outputPoints.Parameter.Value

	paramMaker := "MAKER_TABLE"
	outputMaker, err := utility.GetParameterValue(awsSession, paramMaker)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error getting maker table parameter store"),
		}, nil
	}
	MAKER_TABLE := *outputMaker.Parameter.Value

	//calling create maker request to dynamo func
	res, err := CreateMakerRequest(request, MAKER_TABLE, USER_TABLE, POINTS_TABLE, dynaClient)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string(err.Error()),
		}, nil
	}

	body, _ := json.Marshal(res)

	stringBody := string(body)

	return events.APIGatewayProxyResponse{
		Body:       stringBody,
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}

func CreateMakerRequest(req events.APIGatewayProxyRequest, makerTableName, userTableName, pointsTableName string, dynaClient dynamodbiface.DynamoDBAPI) (
	[]types.ReturnMakerRequest, error) {
	var postMakerRequest types.NewMakerRequest

	//marshall body to maker request struct
	if err := json.Unmarshal([]byte(req.Body), &postMakerRequest); err != nil {
		return nil, errors.New(types.ErrorInvalidMakerData)
	}

	if postMakerRequest.MakerUUID == "" {
		err := errors.New(types.ErrorInvalidMakerData)
		return nil, err
	}

	_, err := FetchUserByID(postMakerRequest.MakerUUID, req, userTableName, dynaClient)
	if err != nil {
		return nil, errors.New(types.ErrorUserDoesNotExist)
	}

	if postMakerRequest.ResourceType == "user" {
		//marshall body to point struct
		var userData types.User
		if err := json.Unmarshal(postMakerRequest.RequestData, &userData); err != nil {
			return nil, errors.New(types.ErrorCouldNotMarshalItem)
		}

		// check if user exist
		_, err = FetchUserByID(userData.User_ID, req, userTableName, dynaClient)
		if err != nil {
			return nil, errors.New(userData.User_ID)
		}
		// send out email
		for _, role := range postMakerRequest.CheckerRoles {
			users, err := FetchUsersByRoles(role, req, userTableName, dynaClient)
			if err != nil {
				return nil, errors.New(types.ErrorFailedToFetchRecord)
			}
			if len(users) > 0 {
				for _, user := range users {
					err := sendEmail(user.Email, req, dynaClient)
					if err != nil {
						log.Println("error sending email")
					}
				}
			}
		}

		// write to db
		makerRequests := utility.DeconstructPostMakerRequest(postMakerRequest)
		roleCount := len(postMakerRequest.CheckerRoles)
		return utility.BatchWriteToDynamoDB(roleCount, makerRequests, makerTableName, dynaClient)

	} else if postMakerRequest.ResourceType == "points" {

		//marshall body to point struct
		var pointsData types.UserPoint
		if err := json.Unmarshal(postMakerRequest.RequestData, &pointsData); err != nil {
			return nil, errors.New(types.ErrorCouldNotMarshalItem)
		}
		// check if points exist
		_, err = FetchUserPoint(pointsData.User_ID, req, pointsTableName, dynaClient)
		if err != nil {
			return nil, errors.New(types.ErrorPointsDoesNotExist)
		}

		if pointsData.Points_ID == "" {
			return nil, errors.New(types.ErrorInvalidPointsID)
		}

		// send out email
		for _, role := range postMakerRequest.CheckerRoles {
			users, err := FetchUsersByRoles(role, req, userTableName, dynaClient)
			if err != nil {
				return nil, errors.New(types.ErrorFailedToFetchRecord)
			}
			if len(users) > 0 {
				for _, user := range users {
					err := sendEmail(user.Email, req, dynaClient)
					if err != nil {
						log.Println("error sending email")
					}
				}
			}
		}

		// write to  db
		makerRequests := utility.DeconstructPostMakerRequest(postMakerRequest)
		roleCount := len(postMakerRequest.CheckerRoles)
		return utility.BatchWriteToDynamoDB(roleCount, makerRequests, makerTableName, dynaClient)
	}

	return nil, errors.New(types.ErrorInvalidResourceType)
}

func FetchUserByID(id string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*types.User, error) {
	//get single user from dynamo
	input := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"user_id": {
				S: aws.String(id),
			},
		},
		TableName: aws.String(tableName),
	}

	result, err := dynaClient.GetItem(input)

	if err != nil {
		return nil, errors.New(types.ErrorFailedToFetchRecordID)
	}
	item := new(types.User)
	err = dynamodbattribute.UnmarshalMap(result.Item, item)
	if err != nil {
		return nil, errors.New(types.ErrorFailedToUnmarshalRecord)
	}

	return item, nil
}

func FetchUsersByRoles(role string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) ([]types.User, error) {
	//get users with a certain role
	input := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("role-index"),
		KeyConditionExpression: aws.String("#role = :role"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":role": {S: aws.String(role)},
		},
		ExpressionAttributeNames: map[string]*string{
			"#role": aws.String("role"),
		},
	}

	result, err := dynaClient.Query(input)

	if err != nil {
		return nil, errors.New(types.ErrorFailedToFetchRecordID)
	}
	users := new([]types.User)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, users)
	if err != nil {
		return nil, errors.New(types.ErrorFailedToUnmarshalRecord)
	}

	return *users, nil
}

func FetchUserPoint(user_id string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*[]types.UserPoint, error) {
	//getting single single user point
	input := &dynamodb.QueryInput{
		TableName: aws.String(tableName),
		KeyConditions: map[string]*dynamodb.Condition{
			"user_id": {
				ComparisonOperator: aws.String("EQ"),
				AttributeValueList: []*dynamodb.AttributeValue{
					{
						S: aws.String(user_id),
					},
				},
			},
		},
	}

	result, err := dynaClient.Query(input)
	if err != nil {
		return nil, errors.New(types.ErrorFailedToFetchRecord)
	}

	if result.Items == nil {
		return nil, errors.New("user point does not exist")
	}

	item := new([]types.UserPoint)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, item)
	if err != nil {
		return nil, errors.New(types.ErrorFailedToUnmarshalRecord)
	}

	return item, nil
}

func main() {
	lambda.Start(handler)
}

func sendEmail(recipientEmail string, req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI) error {
	senderEmail := "pesexoh964@glalen.com"

	// Create an SES session
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("ap-southeast-1"), // Replace with your desired AWS region
	})
	if err != nil {
		log.Println(err)
		return err
	}

	svc := ses.New(sess)
	// Check if the recipient's email is verified
	verifyParams := &ses.GetIdentityVerificationAttributesInput{
		Identities: []*string{aws.String(recipientEmail)},
	}

	verifyResult, verifyErr := svc.GetIdentityVerificationAttributes(verifyParams)
	if verifyErr != nil {
		log.Println("Failed to verify recipient email:", verifyErr)
		// You can handle the verification error as needed
		return verifyErr
	}

	verification, exists := verifyResult.VerificationAttributes[recipientEmail]
	if !exists || *verification.VerificationStatus != "Success" {
		log.Printf("Recipient email (%s) is not verified. Skipping email.", recipientEmail)
		// You can choose to log, return an error, or handle it in your application logic
		return nil // Skip sending the email
	}

	// Compose the email message
	subject := "[Auto-Generated] New Maker Request"
	body := `
		New Maker Request
		
		There is a new maker request in the Ascenda Admin Panel. Go to check it out now:
		
		https://itsag2t2.com/
	`

	// Send the email
	_, err = svc.SendEmail(&ses.SendEmailInput{
		Destination: &ses.Destination{
			ToAddresses: []*string{aws.String(recipientEmail)},
		},
		Message: &ses.Message{
			Body: &ses.Body{
				Text: &ses.Content{
					Data: aws.String(body),
				},
			},
			Subject: &ses.Content{
				Data: aws.String(subject),
			},
		},
		Source: aws.String(senderEmail),
	})

	if err != nil {
		log.Printf("Failed to send email: %v", err)
		return err
	}

	log.Printf("Send email to: %v", recipientEmail)
	return nil
}
