-- name: GetUserByID :one
SELECT *
FROM Employee
WHERE okta_id = $1
  AND active = true;

-- name: ListUsers :many
SELECT *
FROM Employee
WHERE active = true
ORDER BY id
LIMIT $1 OFFSET $2;

-- name: GetUserByEmail :one
SELECT *
FROM Employee
WHERE email = $1
  AND active = $2;

-- name: CreateUser :one
INSERT INTO Employee (name,
                      email,
                      okta_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: DeactivateUser :one
UPDATE Employee
SET active = false
WHERE okta_id = $1
RETURNING *;

-- name: UpdateUser :one
UPDATE Employee
SET name   = $2,
    email  = $3,
    active = $4
WHERE okta_id = $1
RETURNING *;

-- name: ListGroups :many
SELECT g.name                                                             AS group_name,
       g.okta_id                                                          AS group_okta_id,
       json_agg(json_build_object('OktaID', e.okta_id, 'Email', e.email)) AS members
FROM OktaGroup g
         LEFT JOIN EmployeeOktaGroup eog ON g.name = eog.okta_group_name
         LEFT JOIN Employee e ON eog.employee_id = e.okta_id
GROUP BY g.name, g.okta_id
ORDER BY g.name
LIMIT $1 OFFSET $2;

-- name: GetGroupByID :one
SELECT g.name                                                             AS group_name,
       g.okta_id                                                          AS group_okta_id,
       json_agg(json_build_object('OktaID', e.okta_id, 'Email', e.email)) AS members
FROM OktaGroup g
         LEFT JOIN EmployeeOktaGroup eog ON g.name = eog.okta_group_name
         LEFT JOIN Employee e ON e.okta_id = eog.employee_id
WHERE g.okta_id = $1
GROUP BY g.name, g.okta_id;

-- name: GetGroupByName :one
SELECT g.name                                                             AS group_name,
       g.okta_id                                                          AS group_okta_id,
       json_agg(json_build_object('OktaID', e.okta_id, 'Email', e.email)) AS members
FROM OktaGroup g
         LEFT JOIN EmployeeOktaGroup eog ON g.name = eog.okta_group_name
         LEFT JOIN Employee e ON e.okta_id = eog.employee_id
WHERE g.name = $1
GROUP BY g.name, g.okta_id;

-- name: CreateGroup :one
WITH inserted AS (
    INSERT INTO OktaGroup (name, okta_id)
        VALUES ($1, $2)
        ON CONFLICT (name) DO NOTHING
        RETURNING *)
SELECT *
FROM inserted
UNION
SELECT *
FROM OktaGroup
WHERE name = $1;

-- name: CreateEmployeeOktaGroup :exec
INSERT INTO EmployeeOktaGroup (employee_id, okta_group_name)
SELECT e.id, og.id
FROM Employee e,
     OktaGroup og
WHERE e.okta_id = $1
  AND og.name = $2
ON CONFLICT (employee_id, okta_group_name) DO NOTHING;

-- name: UpdateGroupName :one
UPDATE OktaGroup
SET name = $2
WHERE okta_id = $1
RETURNING *;

-- name: UpdateGroupOktaID :one
UPDATE OktaGroup
SET okta_id = $2
WHERE name = $1
RETURNING *;

-- name: RemoveAllGroupMembers :exec
DELETE
FROM EmployeeOktaGroup
WHERE okta_group_name = $1;

-- name: AddGroupMember :exec
INSERT INTO EmployeeOktaGroup (employee_id, okta_group_name)
VALUES ($1, $2)
ON CONFLICT (employee_id, okta_group_name) DO NOTHING;

-- name: DeleteGroup :exec
DELETE
FROM OktaGroup
WHERE okta_id = $1;

-- name: DeleteGroupMembers :exec
DELETE
FROM EmployeeOktaGroup
WHERE okta_group_name IN (SELECT name
                          FROM OktaGroup
                          WHERE okta_id = $1);