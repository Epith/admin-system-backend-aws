aws dynamodb put-item \
  --table-name roles \
  --item '{
    "owner": {
      "M": {
        "users": {"S": "11110"},
        "points": {"S": "11100"},
        "logs": {"S": "10000"},
        "maker": {"S": "00000"},
        "checker": {"S": "11000"},
        "roles": {"S": "11110"}
      }
    },
    "manager": {
      "M": {
        "users": {"S": "11100"},
        "points": {"S": "11100"},
        "logs": {"S": "10000"},
        "maker": {"S": "11110"},
        "checker": {"S": "11000"},
        "roles": {"S": "10000"}
      }
    },
    "engineer": {
      "M": {
        "users": {"S": "10000"},
        "points": {"S": "10000"},
        "logs": {"S": "10000"},
        "maker": {"S": "11110"},
        "checker": {"S": "10000"},
        "roles": {"S": "10000"}
      }
    },
    "product manager": {
      "M": {
        "users": {"S": "10000"},
        "points": {"S": "10000"},
        "logs": {"S": "00000"},
        "maker": {"S": "11110"},
        "checker": {"S": "10000"},
        "roles": {"S": "10000"}
      }
    }
  }'