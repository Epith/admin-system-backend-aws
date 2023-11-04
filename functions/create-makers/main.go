package main

import (
	"ascenda/functions/utility"
	"encoding/json"
	"errors"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/google/uuid"
)

var (
	ErrorCouldNotMarshalItem     = "could not marshal item"
	ErrorCouldNotDynamoPutItem   = "could not dynamo put item"
	ErrorInvalidMakerData        = "invalid maker data"
	ErrorInvalidPointsID         = "invalid points id"
	ErrorInvalidResourceType     = "resource type is invalid"
	ErrorUserDoesNotExist        = "target user does not exist"
	ErrorPointsDoesNotExist      = "target points does not exist"
	ErrorFailedToUnmarshalRecord = "failed to unmarshal record"
	ErrorFailedToFetchRecord     = "failed to fetch record"
	ErrorFailedToFetchRecordID   = "failed to fetch record by uuid"
)

type UserPoint struct {
	User_ID   string `json:"user_id"`
	Points_ID string `json:"points_id"`
	Points    int    `json:"points"`
}

type User struct {
	Email     string `json:"email"`
	User_ID   string `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
}

type Log struct {
	Log_ID          string      `json:"log_id"`
	Severity        int         `json:"severity"`
	User_ID         string      `json:"user_id"`
	Action_Type     int         `json:"action_type"`
	Resource_Type   string      `json:"resource_type"`
	Body            interface{} `json:"body"`
	QueryParameters interface{} `json:"query_parameters"`
	Error           interface{} `json:"error"`
	Timestamp       time.Time   `json:"timestamp"`
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//getting variables
	region := os.Getenv("AWS_REGION")
	USER_TABLE := os.Getenv("USER_TABLE")
	POINTS_TABLE := os.Getenv("POINTS_TABLE")
	MAKER_TABLE := os.Getenv("MAKER_TABLE")

	//setting up dynamo session
	awsSession, err := session.NewSession(&aws.Config{
		Region: aws.String(region)})

	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error setting up aws session"),
		}, nil
	}
	dynaClient := dynamodb.New(awsSession)

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
	[]utility.ReturnMakerRequest, error) {
	var postMakerRequest utility.NewMakerRequest

	//marshall body to maker request struct
	if err := json.Unmarshal([]byte(req.Body), &postMakerRequest); err != nil {
		if logErr := sendLogs(req, 2, 2, "maker", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorInvalidMakerData)
	}

	if postMakerRequest.MakerUUID == "" {
		err := errors.New(ErrorInvalidMakerData)
		if logErr := sendLogs(req, 2, 2, "maker", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, err
	}

	_, err := FetchUserByID(postMakerRequest.MakerUUID, req, userTableName, dynaClient)
	if err != nil {
		if logErr := sendLogs(req, 2, 1, "maker", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorUserDoesNotExist)
	}

	if postMakerRequest.ResourceType == "user" {
		//marshall body to point struct
		var userData User
		if err := json.Unmarshal(postMakerRequest.RequestData, &userData); err != nil {
			if logErr := sendLogs(req, 2, 2, "maker", dynaClient, err); logErr != nil {
				log.Println("Logging err :", logErr)
			}
			return nil, errors.New(ErrorCouldNotMarshalItem)
		}

		// check if user exist
		_, err = FetchUserByID(userData.User_ID, req, userTableName, dynaClient)
		if err != nil {
			if logErr := sendLogs(req, 2, 1, "maker", dynaClient, err); logErr != nil {
				log.Println("Logging err :", logErr)
			}
			return nil, errors.New(userData.User_ID)
		}
		// send out email
		for _, role := range postMakerRequest.CheckerRoles {
			users, err := FetchUsersByRoles(role, req, userTableName, dynaClient)
			if err != nil {
				if logErr := sendLogs(req, 2, 2, "maker", dynaClient, err); logErr != nil {
					log.Println("Logging err :", logErr)
				}
				return nil, errors.New(ErrorFailedToFetchRecord)
			}
			if len(users) > 0 {
				for _, user := range users {
					err := sendEmail(user.Email, req, dynaClient)
					if err != nil {
						if logErr := sendLogs(req, 3, 2, "maker", dynaClient, err); logErr != nil {
							log.Println("Logging err :", logErr)
						}
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
		var pointsData UserPoint
		if err := json.Unmarshal(postMakerRequest.RequestData, &pointsData); err != nil {
			if logErr := sendLogs(req, 2, 2, "maker", dynaClient, err); logErr != nil {
				log.Println("Logging err :", logErr)
			}
			return nil, errors.New(ErrorCouldNotMarshalItem)
		}
		// check if points exist
		_, err = FetchUserPoint(pointsData.User_ID, req, pointsTableName, dynaClient)
		if err != nil {
			if logErr := sendLogs(req, 2, 1, "maker", dynaClient, err); logErr != nil {
				log.Println("Logging err :", logErr)
			}
			return nil, errors.New(ErrorPointsDoesNotExist)
		}

		if pointsData.Points_ID == "" {
			if logErr := sendLogs(req, 2, 2, "maker", dynaClient, err); logErr != nil {
				log.Println("Logging err :", logErr)
			}
			return nil, errors.New(ErrorInvalidPointsID)
		}

		// send out email
		for _, role := range postMakerRequest.CheckerRoles {
			users, err := FetchUsersByRoles(role, req, userTableName, dynaClient)
			if err != nil {
				return nil, errors.New(ErrorFailedToFetchRecord)
			}
			if len(users) > 0 {
				for _, user := range users {
					err := sendEmail(user.Email, req, dynaClient)
					if err != nil {
						if logErr := sendLogs(req, 3, 2, "maker", dynaClient, err); logErr != nil {
							log.Println("Logging err :", logErr)
						}
					}
				}
			}
		}

		// write to  db
		makerRequests := utility.DeconstructPostMakerRequest(postMakerRequest)
		roleCount := len(postMakerRequest.CheckerRoles)
		return utility.BatchWriteToDynamoDB(roleCount, makerRequests, makerTableName, dynaClient)
	}

	return nil, errors.New(ErrorInvalidResourceType)
}

func FetchUserByID(id string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*User, error) {
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
		if logErr := sendLogs(req, 3, 1, "user", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorFailedToFetchRecordID)
	}
	item := new(User)
	err = dynamodbattribute.UnmarshalMap(result.Item, item)
	if err != nil {
		if logErr := sendLogs(req, 3, 1, "user", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorFailedToUnmarshalRecord)
	}

	if logErr := sendLogs(req, 1, 1, "user", dynaClient, err); logErr != nil {
		log.Println("Logging err :", logErr)
	}
	return item, nil
}

func FetchUsersByRoles(role string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) ([]User, error) {
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
		if logErr := sendLogs(req, 3, 1, "user", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorFailedToFetchRecordID)
	}
	users := new([]User)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, users)
	if err != nil {
		if logErr := sendLogs(req, 3, 1, "maker", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorFailedToUnmarshalRecord)
	}

	if logErr := sendLogs(req, 1, 1, "maker", dynaClient, err); logErr != nil {
		log.Println("Logging err :", logErr)
	}
	return *users, nil
}

func FetchUserPoint(user_id string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*[]UserPoint, error) {
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
		if logErr := sendLogs(req, 3, 1, "point", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorFailedToFetchRecord)
	}

	if result.Items == nil {
		return nil, errors.New("user point does not exist")
	}

	item := new([]UserPoint)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, item)
	if err != nil {
		if logErr := sendLogs(req, 3, 1, "point", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorFailedToUnmarshalRecord)
	}

	if logErr := sendLogs(req, 1, 1, "point", dynaClient, err); logErr != nil {
		log.Println("Logging err :", logErr)
	}

	return item, nil
}

func main() {
	lambda.Start(handler)
}

func sendLogs(req events.APIGatewayProxyRequest, severity int, action int, resource string, dynaClient dynamodbiface.DynamoDBAPI, err error) error {
	LOGS_TABLE := os.Getenv("LOGS_TABLE")
	//create log struct
	log := Log{}
	log.Body = RemoveNewlineAndUnnecessaryWhitespace(req.Body)
	log.QueryParameters = req.QueryStringParameters
	log.Error = err
	log.Log_ID = uuid.NewString()
	log.Severity = severity
	log.User_ID = req.RequestContext.Identity.User
	log.Action_Type = action
	log.Resource_Type = resource
	log.Timestamp = time.Now().UTC()

	av, err := dynamodbattribute.MarshalMap(log)

	if err != nil {
		return errors.New("failed to marshal log")
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(LOGS_TABLE),
	}
	_, err = dynaClient.PutItem(input)
	if err != nil {
		return errors.New("Could not dynamo put")
	}
	return nil
}

func RemoveNewlineAndUnnecessaryWhitespace(body string) string {
	// Remove newline characters
	body = regexp.MustCompile(`\n|\r`).ReplaceAllString(body, "")

	// Remove unnecessary whitespace
	body = regexp.MustCompile(`\s{2,}|\t`).ReplaceAllString(body, " ")

	// Remove the character `\"`
	body = regexp.MustCompile(`\"`).ReplaceAllString(body, "")

	// Trim the body
	body = strings.TrimSpace(body)

	return body
}

func sendEmail(recipientEmail string, req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI) (error) {
	senderEmail := "ryan.peh.2021@scis.smu.edu.sg"

	// Create an SES session
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("ap-southeast-1"), // Replace with your desired AWS region
	})
	if err != nil {
		log.Println(err)
		return err
	}

	log.Println(123)
	svc := ses.New(sess)

	log.Println(223)
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
	
	log.Println(323)
	if err != nil {
		if logErr := sendLogs(req, 3, 2, "maker", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		log.Printf("Failed to send email: %v", err)
		return err
	}

	log.Printf("Send email to: %v", recipientEmail)
	return  nil
}
