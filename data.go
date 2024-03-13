package main

import (
	"strings"
	"time"
)

type SCIMUser struct {
	Schemas  []string    `json:"schemas"`
	ID       string      `json:"id"`
	UserName string      `json:"userName"`
	Name     SCIMName    `json:"name"`
	Active   bool        `json:"active"`
	Emails   []SCIMEmail `json:"emails"`
	Groups   []string    `json:"groups"`
	Meta     SCIMMeta    `json:"meta"`
}

type SCIMName struct {
	GivenName  string `json:"givenName"`
	MiddleName string `json:"middleName"`
	FamilyName string `json:"familyName"`
}

type SCIMEmail struct {
	Primary bool   `json:"primary"`
	Value   string `json:"value"`
	Type    string `json:"type"`
	Display string `json:"display"`
}

type SCIMMeta struct {
	ResourceType string    `json:"resourceType"`
	Created      time.Time `json:"created,omitempty"`
	LastModified time.Time `json:"lastModified,omitempty"`
	Version      string    `json:"version,omitempty"`
}

type SCIMError struct {
	Schemas []string `json:"schemas"`
	Detail  string   `json:"detail"`
	Status  string   `json:"status"`
}

type SCIMUserCreateRequest struct {
	Schemas     []string        `json:"schemas"`
	UserName    string          `json:"userName"`
	Name        SCIMUserName    `json:"name"`
	Emails      []SCIMUserEmail `json:"emails"`
	DisplayName string          `json:"displayName"`
	Locale      string          `json:"locale"`
	ExternalID  string          `json:"externalId"`
	Password    string          `json:"password"`
	Active      bool            `json:"active"`
}

type SCIMUserName struct {
	GivenName  string `json:"givenName"`
	FamilyName string `json:"familyName"`
}

type SCIMUserEmail struct {
	Value   string `json:"value"`
	Type    string `json:"type"`
	Primary bool   `json:"primary"`
}

type SCIMUserUpdate struct {
	Schemas  []string `json:"schemas"`
	ID       string   `json:"id"`
	UserName string   `json:"userName"`
	Name     struct {
		GivenName  string `json:"givenName"`
		MiddleName string `json:"middleName"`
		FamilyName string `json:"familyName"`
	} `json:"name"`
	Emails []struct {
		Primary bool   `json:"primary"`
		Value   string `json:"value"`
		Type    string `json:"type"`
		Display string `json:"display"`
	} `json:"emails"`
	Active bool `json:"active"`
}

type SCIMGroupCreateRequest struct {
	Schemas     []string `json:"schemas"`
	DisplayName string   `json:"displayName"`
	Members     []struct {
		Value   string `json:"value"`
		Display string `json:"display"`
	} `json:"members"`
}

// SCIMGroup represents a SCIM Group object following the SCIM 2.0 specification.
type SCIMGroup struct {
	Schemas     []string          `json:"schemas"`     // Must include "urn:ietf:params:scim:schemas:core:2.0:Group"
	ID          string            `json:"id"`          // The unique identifier for the SCIM resource as defined by the Service Provider
	DisplayName string            `json:"displayName"` // A human-readable name for the Group
	Members     []SCIMGroupMember `json:"members"`     // A list of members in the Group
	Meta        SCIMGroupMeta     `json:"meta"`        // A complex attribute containing resource metadata
}

// SCIMGroupMember represents a member of the SCIM Group.
type SCIMGroupMember struct {
	Value   string `json:"value"`   // The identifier of the member in this Group
	Display string `json:"display"` // A human-readable name for the member, primarily used for display purposes
}

// SCIMGroupMeta contains metadata about the SCIM Group resource.
type SCIMGroupMeta struct {
	ResourceType string `json:"resourceType"`           // The name of the resource type of the resource
	Created      string `json:"created,omitempty"`      // The DateTime the resource was added to the service provider
	LastModified string `json:"lastModified,omitempty"` // The most recent DateTime the details of this resource were updated
	Location     string `json:"location,omitempty"`     // The URI of the resource being returned
	Version      string `json:"version,omitempty"`      // The version of the resource being returned
	// Additional metadata fields can be included as per requirement
}

type SCIMGroupUpdateRequest struct {
	Schemas     []string          `json:"schemas"`
	ID          string            `json:"id"`
	DisplayName string            `json:"displayName"`
	Members     []SCIMGroupMember `json:"members"`
}

type User struct {
	ID     int32
	Name   string
	Email  string
	OktaID string
}

type Group struct {
	ID      string
	Name    string
	OktaID  string
	Members []User
}

func convertToSCIMUser(dbUser *User) SCIMUser {
	names := strings.Fields(dbUser.Name)
	givenName := names[0]
	familyName := ""
	if len(names) > 1 {
		familyName = strings.Join(names[1:], " ")
	}

	return SCIMUser{
		Schemas:  []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		ID:       dbUser.OktaID,
		UserName: dbUser.Email,
		Name: SCIMName{
			GivenName:  givenName,
			MiddleName: "",
			FamilyName: familyName,
		},
		Active: true,
		Emails: []SCIMEmail{
			{
				Primary: true,
				Value:   dbUser.Email,
				Type:    "work",
				Display: dbUser.Email,
			},
		},
		Groups: []string{},
		Meta: SCIMMeta{
			ResourceType: "User",
		},
	}
}

func convertToSCIMGroup(group *Group) SCIMGroup {
	// Initialize an empty slice for SCIM members
	var members []SCIMGroupMember

	for _, member := range group.Members {
		scimMember := SCIMGroupMember{
			Value:   member.OktaID,
			Display: member.Email,
		}
		members = append(members, scimMember)
	}

	// Construct the SCIM group
	scimGroup := SCIMGroup{
		Schemas:     []string{"urn:ietf:params:scim:schemas:core:2.0:Group"},
		ID:          group.OktaID,
		DisplayName: group.Name,
		Members:     members,
		Meta: SCIMGroupMeta{
			ResourceType: "Group",
		},
	}

	return scimGroup
}
