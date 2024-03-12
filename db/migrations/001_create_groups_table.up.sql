-- Create the OktaGroup table
CREATE TABLE IF NOT EXISTS OktaGroup
(
    id      SERIAL PRIMARY KEY,
    name    VARCHAR(255) UNIQUE NOT NULL,
    okta_id VARCHAR(255) UNIQUE
);

-- Create the Employee table
CREATE TABLE IF NOT EXISTS Employee
(
    id      SERIAL PRIMARY KEY,
    name    VARCHAR(255)        NOT NULL,
    email   VARCHAR(255) UNIQUE NOT NULL,
    okta_id VARCHAR(255) UNIQUE NOT NULL,
    active  BOOLEAN             NOT NULL DEFAULT TRUE
);

-- Create the intermediate table for many-to-many relationship
CREATE TABLE IF NOT EXISTS EmployeeOktaGroup
(
    employee_id     VARCHAR(255) NOT NULL,
    okta_group_name VARCHAR(255) NOT NULL,
    PRIMARY KEY (employee_id, okta_group_name),
    FOREIGN KEY (employee_id) REFERENCES Employee (okta_id) ON DELETE CASCADE,
    FOREIGN KEY (okta_group_name) REFERENCES OktaGroup (name) ON DELETE CASCADE
);

CREATE TYPE EmployeeDetails AS
(
    okta_id VARCHAR,
    email   VARCHAR
);