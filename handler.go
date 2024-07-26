package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"main/db"
)

type handler struct {
	username   string
	password   string
	logger     *log.Logger
	db         *db.Queries
	dbConn     *sql.DB
	oktaClient *oktaClient
}

func NewHandler(username, password string, logger *log.Logger, db *db.Queries, dbConn *sql.DB, oktaClient *oktaClient) *handler {
	return &handler{
		username:   username,
		password:   password,
		logger:     logger,
		db:         db,
		dbConn:     dbConn,
		oktaClient: oktaClient,
	}
}

func (h *handler) applyMiddlewares(handle httprouter.Handle) httprouter.Handle {
	return h.basicAuth(h.loggingMiddleware(handle))
}

func (h *handler) basicAuth(handle httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != h.username || pass != h.password {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		handle(w, r, ps)
	}
}

func (h *handler) loggingMiddleware(handle httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Save the current time to calculate the request duration later
		startTime := time.Now()

		// Read the request body
		bodyBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			h.logger.Printf("Error reading request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Restore the request body for future use in the chain
		r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

		// Use a ResponseWriter that allows us to capture the status code and body size
		rec := httptest.NewRecorder()
		handle(rec, r, ps)

		// Copy everything from the recorder to the actual response writer
		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		rec.Body.WriteTo(w)

		// Calculate request duration
		duration := time.Since(startTime)

		// Log in Nginx combined log format with request body appended
		h.logger.Printf(`%s - - [%s] "%s %s %s" %d %d "%s" "%s" "%s" %v`,
			r.RemoteAddr,
			startTime.Format("02/Jan/2006:15:04:05 -0700"),
			r.Method,
			r.URL.RequestURI(),
			r.Proto,
			rec.Code,
			rec.Body.Len(),
			r.Referer(),
			r.UserAgent(),
			string(bodyBytes),
			duration,
		)
	}
}

func (h *handler) extractOktaID(encodedID string) string {
	pattern := `%!d\(string=(.+)\)`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(encodedID)

	if len(matches) == 2 {
		return matches[1]
	} else {
		return encodedID
	}
}

func (h *handler) GetUser() httprouter.Handle {
	return h.applyMiddlewares(func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Extract the user ID from the path parameters
		encodedID := ps.ByName("id")

		// Attempt to extract the actual ID from the encoded format
		oktaID := h.extractOktaID(encodedID)

		user, err := h.db.GetUserByID(context.Background(), oktaID)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "User not found", http.StatusNotFound)
			} else {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		scimUser := convertToSCIMUser(
			&User{
				ID:     user.ID,
				Name:   user.Name,
				Email:  user.Email,
				OktaID: oktaID,
			},
		)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(scimUser); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

func (h *handler) UpdateUser() httprouter.Handle {
	return h.applyMiddlewares(func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		userID := ps.ByName("id")

		var updateUserReq SCIMUserUpdate
		if err := json.NewDecoder(r.Body).Decode(&updateUserReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		updatedUser, err := h.db.UpdateUser(context.Background(), db.UpdateUserParams{
			OktaID: userID,
			Name:   updateUserReq.Name.GivenName + " " + updateUserReq.Name.FamilyName,
			Email:  updateUserReq.Emails[0].Value,
			Active: updateUserReq.Active,
		})
		if err != nil {
			// Handle errors, e.g., user not found or database errors
			http.Error(w, "Failed to update user", http.StatusInternalServerError)
			return
		}

		// Convert the updated database user model to a SCIM user model here
		scimUser := convertToSCIMUser(&User{
			ID:     updatedUser.ID,
			Name:   updatedUser.Name,
			Email:  updatedUser.Email,
			OktaID: userID,
		})

		// Set response header
		w.Header().Set("Content-Type", "application/json")

		// Respond with the updated user object
		if err := json.NewEncoder(w).Encode(scimUser); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

func (h *handler) DeactivateUser() httprouter.Handle {
	return h.applyMiddlewares(func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Extract the user ID from the path parameters
		userID := ps.ByName("id")

		// Deactivate the user by setting the Active attribute to false
		_, err := h.db.DeactivateUser(context.Background(), userID)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "User not found", http.StatusNotFound)
			} else {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		// Respond with a success status code
		w.WriteHeader(http.StatusOK)
	})
}

func (h *handler) GetUsers() httprouter.Handle {
	return h.applyMiddlewares(func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		query := r.URL.Query()
		filter := query.Get("filter")
		if filter != "" {
			h.FindUser(w, r, ps)
		} else {
			h.ListUsers(w, r, ps)
		}
	})
}

func (h *handler) ListUsers(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Set default values for pagination
	var startIndex int = 1 // Pagination starts at 1
	var count int = 100    // Default count

	// Parse startIndex and count from query parameters, if present
	if start := r.URL.Query().Get("startIndex"); start != "" {
		if s, err := strconv.Atoi(start); err == nil && s > 0 {
			startIndex = s
		}
	}

	if c := r.URL.Query().Get("count"); c != "" {
		if c, err := strconv.Atoi(c); err == nil && c > 0 {
			count = c
		}
	}

	// Adjust startIndex for SQL OFFSET which starts at 0
	offset := startIndex - 1

	// Fetch paginated list of users from the database
	dbUsers, err := h.db.ListUsers(context.Background(), db.ListUsersParams{int32(count), int32(offset)})
	if err != nil {
		http.Error(w, "Failed to retrieve users", http.StatusInternalServerError)
		return
	}

	// Convert users from DB format to SCIM format
	scimUsers := make([]SCIMUser, len(dbUsers))
	for i, dbUser := range dbUsers {
		scimUsers[i] = convertToSCIMUser(
			&User{
				ID:     dbUser.ID,
				Name:   dbUser.Name,
				Email:  dbUser.Email,
				OktaID: dbUser.OktaID,
			},
		)
	}

	// Construct SCIM response with list of users
	scimResponse := struct {
		Schemas      []string   `json:"schemas"`
		TotalResults int        `json:"totalResults"`
		StartIndex   int        `json:"startIndex"`
		ItemsPerPage int        `json:"itemsPerPage"`
		Resources    []SCIMUser `json:"Resources"`
	}{
		Schemas:      []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"},
		TotalResults: len(scimUsers), // This should ideally be the total count from the DB, not just the count of this page
		StartIndex:   startIndex,
		ItemsPerPage: len(scimUsers),
		Resources:    scimUsers,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(scimResponse); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *handler) FindUser(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	query := r.URL.Query()
	filter := query.Get("filter")

	var userName string
	_, err2 := fmt.Sscanf(filter, "userName eq %q", &userName)
	if err2 != nil {
		http.Error(w, "Invalid or missing filter query", http.StatusBadRequest)
		return
	}

	if userName == "" {
		http.Error(w, "Invalid or missing filter query", http.StatusBadRequest)
		return
	}

	user, err := h.db.GetUserByEmail(context.Background(), db.GetUserByEmailParams{
		Email:  userName,
		Active: true,
	})
	if err != nil {
		// Return an empty SCIM list response when the user is not found
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // Set HTTP status to 200 OK
		json.NewEncoder(w).Encode(struct {
			Schemas      []string      `json:"schemas"`
			TotalResults int           `json:"totalResults"`
			StartIndex   int           `json:"startIndex"`
			ItemsPerPage int           `json:"itemsPerPage"`
			Resources    []interface{} `json:"Resources"`
		}{
			Schemas:      []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"},
			TotalResults: 0,
			StartIndex:   1,
			ItemsPerPage: 0,
			Resources:    []interface{}{},
		})
		return
	}

	scimUser := convertToSCIMUser(
		&User{
			ID:     user.ID,
			Name:   user.Name,
			Email:  user.Email,
			OktaID: user.OktaID,
		},
	)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scimUser)
}

func (h *handler) CreateUser() httprouter.Handle {
	return h.applyMiddlewares(func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		var req SCIMUserCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Check if user already exists based on userName or email
		exists, err := h.db.GetUserByEmail(context.Background(), db.GetUserByEmailParams{
			Email:  req.UserName,
			Active: false,
		})
		if err != nil && err != sql.ErrNoRows {
			http.Error(w, "Error checking user existence", http.StatusInternalServerError)
			return
		}
		if exists.ID > 0 && exists.Active {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(SCIMError{
				Schemas: []string{"urn:ietf:params:scim:api:messages:2.0:Error"},
				Detail:  "User already exists",
				Status:  "409",
			})
			return
		}

		var user db.Employee

		if exists.ID == 0 {
			user, err = h.db.CreateUser(context.Background(), db.CreateUserParams{
				Email:  req.UserName,
				Name:   req.Name.GivenName + " " + req.Name.FamilyName,
				OktaID: req.ExternalID,
			})
			if err != nil {
				fmt.Errorf("Error creating user: %v", err)
				http.Error(w, "Error creating user", http.StatusInternalServerError)
				return
			}
		} else {
			user, err = h.db.UpdateUser(context.Background(), db.UpdateUserParams{
				OktaID: req.ExternalID,
				Name:   req.Name.GivenName + " " + req.Name.FamilyName,
				Email:  req.UserName,
				Active: true,
			})
			if err != nil {
				fmt.Errorf("Error updating user: %v", err)
				http.Error(w, "Error updating user", http.StatusInternalServerError)
				return
			}
		}

		// Convert to SCIM user response
		scimUser := convertToSCIMUser(&User{
			ID:     user.ID,
			Name:   user.Name,
			Email:  user.Email,
			OktaID: user.OktaID,
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(scimUser)
	})
}

func (h *handler) CreateGroup() httprouter.Handle {
	return h.applyMiddlewares(func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		var groupReq SCIMGroupCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&groupReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Insert the new group into the database
		newGroup, err := h.db.CreateGroup(context.Background(), db.CreateGroupParams{
			Name:   groupReq.DisplayName,
			OktaID: sql.NullString{String: uuid.New().String(), Valid: true},
		})
		if err != nil {
			http.Error(w, "Failed to create group", http.StatusInternalServerError)
			return
		}

		var members []User

		// For each member in the group request, insert a record into the employeeoktagroup table
		for _, member := range groupReq.Members {
			members = append(members, User{
				OktaID: member.Value,
				Name:   member.Display,
			})
			err := h.db.CreateEmployeeOktaGroup(context.Background(), db.CreateEmployeeOktaGroupParams{
				OktaID: member.Value,
				Name:   newGroup.Name,
			})
			if err != nil {
				// Handle the error, e.g., rollback the transaction, log the error, etc.
				// For simplicity, returning an error response here
				http.Error(w, "Failed to add member to group", http.StatusInternalServerError)
				return
			}
		}

		// Convert the newly created database group model to a SCIM group model
		scimGroup := convertToSCIMGroup(&Group{
			Name:    newGroup.Name,
			OktaID:  newGroup.OktaID.String,
			Members: members,
		})

		// Set response header
		w.Header().Set("Content-Type", "application/json")

		// Respond with the created group object
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(scimGroup); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

func (h *handler) ListGroups() httprouter.Handle {
	return h.applyMiddlewares(func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		// Default values
		startIndex := 1
		count := 100

		// Parse query parameters
		if start, err := strconv.Atoi(r.URL.Query().Get("startIndex")); err == nil && start > 0 {
			startIndex = start
		}
		if c, err := strconv.Atoi(r.URL.Query().Get("count")); err == nil && c > 0 {
			count = c
		}

		// Adjust for SQL OFFSET (0-indexed)
		offset := startIndex - 1

		// Parse filter
		filter := r.URL.Query().Get("filter")
		var err error
		var scimGroups []SCIMGroup

		if filter != "" {
			// Extract the filter value (group name) from the filter query
			var groups []db.GetGroupByNameRow
			var groupName string
			_, err := fmt.Sscanf(filter, "displayName eq %q", &groupName)
			if err != nil {
				http.Error(w, "Invalid filter query", http.StatusBadRequest)
				return
			}

			// Fetch a specific group by name
			group, err := h.db.GetGroupByName(context.Background(), groupName)
			if err != nil {
				if err == sql.ErrNoRows {
					http.Error(w, "No group found", http.StatusNotFound)
				} else {
					http.Error(w, "Internal server error", http.StatusInternalServerError)
				}
				return
			}
			groups = append(groups, group)
			scimGroups = make([]SCIMGroup, len(groups))
			for i, group := range groups {
				var members []User
				if group.Members != nil {
					if err := json.Unmarshal(group.Members, &members); err != nil {
						http.Error(w, "Internal server error", http.StatusInternalServerError)
						return
					}
				}
				scimGroups[i] = convertToSCIMGroup(&Group{
					Name:    group.GroupName,
					OktaID:  group.GroupOktaID.String,
					Members: members,
				})
			}
		} else {
			// Fetch all groups with pagination
			var groups []db.ListGroupsRow
			groups, err = h.db.ListGroups(context.Background(), db.ListGroupsParams{Limit: int32(count), Offset: int32(offset)})
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			scimGroups := make([]SCIMGroup, len(groups))
			for i, group := range groups {
				var members []User
				if group.Members != nil {
					if err := json.Unmarshal(group.Members, &members); err != nil {
						http.Error(w, "Internal server error", http.StatusInternalServerError)
						return
					}
				}
				scimGroups[i] = convertToSCIMGroup(&Group{
					Name:    group.GroupName,
					OktaID:  group.GroupOktaID.String,
					Members: members,
				})
			}
		}

		// Construct and send response
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(scimGroups); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

func (h *handler) GetGroup() httprouter.Handle {
	return h.applyMiddlewares(func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		groupID := ps.ByName("id")

		// Fetch group from the database by ID
		group, err := h.db.GetGroupByID(context.Background(), sql.NullString{String: groupID, Valid: true})
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Group not found", http.StatusNotFound)
			} else {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		var members []User
		if err := json.Unmarshal(group.Members, &members); err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Convert to SCIM format
		scimGroup := convertToSCIMGroup(&Group{
			Name:    group.GroupName,
			OktaID:  group.GroupOktaID.String,
			Members: members,
		})

		// Construct and send response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(scimGroup)
	})
}

func (h *handler) UpdateGroup() httprouter.Handle {
	return h.applyMiddlewares(func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Extract the group ID from the request parameters

		groupID := ps.ByName("id")

		// Decode the request body to get the updated group details
		var updateReq SCIMGroupUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Begin a transaction to ensure atomic updates
		tx, err := h.dbConn.BeginTx(context.Background(), nil)
		if err != nil {
			http.Error(w, "Failed to begin transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		_, err = h.db.UpdateGroupOktaID(context.Background(), db.UpdateGroupOktaIDParams{
			Name:   updateReq.DisplayName,
			OktaID: sql.NullString{String: groupID, Valid: true},
		})
		if err != nil {
			http.Error(w, "Failed to update group", http.StatusInternalServerError)
			return
		}

		// Fetch current group members
		currentMembers, err := h.db.GetGroupMembers(context.Background(), updateReq.DisplayName)
		if err != nil {
			http.Error(w, "Failed to fetch group members", http.StatusInternalServerError)
			return
		}

		// Map current members for easy lookup
		currentMemberMap := make(map[string]bool)
		for _, member := range currentMembers {
			currentMemberMap[member.OktaID] = true
		}

		// Map new members from the update request
		newMemberMap := make(map[string]SCIMGroupMember)
		for _, member := range updateReq.Members {
			if member.Display == "" || member.Value == "" {
				continue
			}
			newMemberMap[member.Value] = member
		}

		// Determine members to add and remove
		var membersToAdd []string
		var membersToRemove []string
		for value, member := range newMemberMap {
			if !currentMemberMap[value] {
				membersToAdd = append(membersToAdd, member.Value)
			}
		}
		for value := range currentMemberMap {
			if _, exists := newMemberMap[value]; !exists {
				membersToRemove = append(membersToRemove, value)
			}
		}

		fmt.Printf("updateReq: %v\n", updateReq.DisplayName)
		fmt.Printf("membersToAdd: %v\n", membersToAdd)
		fmt.Printf("membersToRemove: %v\n", membersToRemove)

		// Add new members
		for _, member := range membersToAdd {
			if err := h.db.AddGroupMember(context.Background(), db.AddGroupMemberParams{
				EmployeeID:    member,
				OktaGroupName: updateReq.DisplayName,
			}); err != nil {
				http.Error(w, "Failed to add group member", http.StatusInternalServerError)
				return
			}
		}

		// Remove members no longer in the group
		for _, value := range membersToRemove {
			if err := h.db.RemoveGroupMember(context.Background(), db.RemoveGroupMemberParams{
				EmployeeID:    value,
				OktaGroupName: updateReq.DisplayName,
			}); err != nil {
				http.Error(w, "Failed to remove group member", http.StatusInternalServerError)
				return
			}
		}

		// Commit the transaction
		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		// Fetch the updated group details and members
		updatedGroupDetails, err := h.db.GetGroupByName(context.Background(), updateReq.DisplayName)
		if err != nil {
			http.Error(w, "Failed to fetch updated group details", http.StatusInternalServerError)
			return
		}

		var members []User
		if err := json.Unmarshal(updatedGroupDetails.Members, &members); err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Construct the SCIM group response with updated details and members
		updatedGroup := convertToSCIMGroup(&Group{
			Name:    updatedGroupDetails.GroupName,
			OktaID:  updatedGroupDetails.GroupOktaID.String,
			Members: members,
		})

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(updatedGroup); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

func (h *handler) DeleteGroup() httprouter.Handle {
	return h.applyMiddlewares(func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		groupID := ps.ByName("id")

		// Begin a transaction
		tx, err := h.dbConn.Begin()
		if err != nil {
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		if err := h.db.DeleteGroupMembers(context.Background(), sql.NullString{String: groupID, Valid: true}); err != nil {
			http.Error(w, "Failed to delete group members", http.StatusInternalServerError)
			return
		}

		if err := h.db.DeleteGroup(context.Background(), sql.NullString{String: groupID, Valid: true}); err != nil {
			http.Error(w, "Failed to delete group", http.StatusInternalServerError)
			return
		}

		// Commit the transaction
		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		// Return a 204 No Content status code
		w.WriteHeader(http.StatusNoContent)
	})
}
