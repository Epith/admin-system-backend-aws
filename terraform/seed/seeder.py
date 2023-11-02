import boto3
import csv
import sys

# Get table name from argument
table_name = sys.argv[1]

if table_name not in ['users', 'points']:
    print('Invalid table name')
    exit(1)

# Set the region
region = 'ap-southeast-1'

# Create a boto3 client for DynamoDB
dynamodb = boto3.resource('dynamodb', region_name=region)

# Get the DynamoDB table
table = dynamodb.Table(table_name)

# Create a batch writer
with table.batch_writer() as batch:

    # Open the CSV file
    with open(f'./data/{table_name}.csv', 'r') as csvfile:
        reader = csv.reader(csvfile)

        # Skip the header row
        next(reader)

        # Iterate over the CSV rows and add them to the batch writer
        if table_name == 'users':
            for row in reader:
                item = {
                    'user_id': row[0],
                    'email': row[1],
                    'first_name': row[2],
                    'last_name': row[3],
                    'role': row[4] if row[4] else "user"
                }
                batch.put_item(Item=item)

        else:
            for row in reader:
                item = {
                    'user_id': row[1],
                    'points_id': row[0],
                    'points': int(row[2])
                }
                batch.put_item(Item=item)

print(f'CSV file loaded successfully into {table_name} table in {region} region')