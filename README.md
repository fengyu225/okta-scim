# okta-scim

This project implements an Okta SCIM v2.0 service that supports user and group provisioning for Okta SAML applications.

## Overview

The okta-scim service meets the [Okta SCIM v2.0 specification](https://developer.okta.com/docs/reference/scim/scim-20/) and allows for user and group management between Okta and an external application. The service acts as a SCIM endpoint that Okta can communicate with to perform CRUD operations on users and groups.

1. Create an Okta SAML app that supports SCIM provisioning
2. Start the okta-scim service
3. Configure the SCIM connection on the Okta SAML app
4. Perform user and group operations through SCIM

## Features

- User provisioning (create, read, update, delete)
- Group provisioning (create, read, update, delete)
- Attribute mapping
- Just-in-time (JIT) user creation

## Prerequisites

- Go 1.x or higher
- [sqlc](https://sqlc.dev/) for database operations
- PostgreSQL (or your preferred database)
- Docker and Docker Compose (for running the database)

## Project Structure
```
├── data.go
├── db
│   ├── db.go
│   ├── migrations
│   │   ├── 001_create_groups_table.down.sql
│   │   └── 001_create_groups_table.up.sql
│   ├── models.go
│   ├── queries
│   │   └── queries.sql
│   └── queries.sql.go
├── docker
│   └── docker-compose.yml
├── go.mod
├── go.sum
├── handler.go
├── main.go
├── okta.go
└── sqlc.yaml
```

## Installation

1. Clone the repository:
```shell
git clone https://github.com/fengyu225/okta-scim.git
cd okta-scim
```

2. Install dependencies:
```shell
make deps
```

3. Build the project:
```shell
make build
```

## Configuration

1. Set up your database:
```shell
make db-start
make db-create
make migrate
```

2. Configure your Okta SAML application to use SCIM provisioning.

3. Set the following environment variables:
- `SCIM_USER`: Username for basic authentication
- `SCIM_PASSWORD`: Password for basic authentication
- `OKTA_DOMAIN`: Okta domain (Optional, for making API calls to Okta, e.g., `dev-123456.okta.com`)
- `OKTA_API_TOKEN`: Okta API token (Optional, for making API calls to Okta) 

## Usage

1. Start the okta-scim service:
```shell
make run
```

2. Configure the SCIM connection in your Okta SAML application:
- SCIM base URL: `http://service-url:8080/scim/v2`
- Authentication method: Basic Auth
- Username: Value of `SCIM_USER`
- Password: Value of `SCIM_PASSWORD`

3. Test the connection and enable provisioning in Okta.

## Development

This project uses `sqlc` for database operations. To regenerate the database code after making changes to the SQL queries:

```shell
make sqlc
```

## Testing

Run the test suite:
```shell
make test
```