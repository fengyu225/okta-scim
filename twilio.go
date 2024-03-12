package main

import (
	"context"
	"fmt"

	"github.com/okta/okta-sdk-golang/v2/okta"
)

var oktaGroupToTwilioRole = map[string]string{
	"twilio-supervisor":                  "supervisor",
	"twilio-agent":                       "agent",
	"twilio-admin":                       "admin",
	"twilio-wfo-data_analyst":            "wfo.data-analyst",
	"twilio-wfo-quality_process_manager": "wfo.quality-process-manager",
	"twilio-wfo-quality_manager":         "wfo.quality-manager",
	"twilio-wfo-data_auditor":            "wfo.data-auditor",
	"twilio-wfo-dashboard_viewer":        "wfo.dashboard-viewer",
	"twilio-wfo-full_access":             "wfo.full-access",
	"twilio-wfo-team_leader":             "wfo.team-leader",
}

const (
	oktaDomain              = "https://hood.oktapreview.com"
	twilioRoleUserAttribute = "twilioFlexUserRolesArray"
)

type oktaClient struct {
	client *okta.Client
}

func newOktaClient(apiToken string) (*oktaClient, error) {
	ctx := context.TODO()

	ctx, client, err := okta.NewClient(ctx, okta.WithOrgUrl(oktaDomain), okta.WithToken(apiToken))
	if err != nil {
		return nil, fmt.Errorf("failed to create Okta client: %v", err)
	}

	return &oktaClient{client: client}, nil
}

func (o *oktaClient) removeTwilioRole(userId, roleToRemove string) error {
	ctx := context.Background()
	user, _, err := o.client.User.GetUser(ctx, userId)
	if err != nil {
		return fmt.Errorf("failed to get user: %v", err)
	}

	userProfiles := (*user.Profile)
	roles, ok := userProfiles[twilioRoleUserAttribute].([]interface{})
	if !ok {
		return fmt.Errorf("twilioFlexUserRoles attribute not found or is not an array")
	}

	var updatedRoles []string
	for _, role := range roles {
		if roleStr, ok := role.(string); ok && roleStr != roleToRemove {
			updatedRoles = append(updatedRoles, roleStr)
		}
	}

	userProfiles[twilioRoleUserAttribute] = updatedRoles
	user.Profile = &userProfiles

	_, _, err = o.client.User.UpdateUser(ctx, userId, *user, nil)
	if err != nil {
		return fmt.Errorf("failed to update user: %v", err)
	}

	return nil
}

func (o *oktaClient) addTwilioRole(userId, roleToAdd string) error {
	ctx := context.Background()
	user, _, err := o.client.User.GetUser(ctx, userId)
	if err != nil {
		return fmt.Errorf("failed to get user: %v", err)
	}

	userProfiles := (*user.Profile)
	roles, ok := userProfiles[twilioRoleUserAttribute].([]interface{})
	if !ok {
		// If the attribute does not exist or is not an array, initialize it
		roles = []interface{}{}
	}

	// Check if the role to add already exists
	for _, role := range roles {
		if roleStr, ok := role.(string); ok && roleStr == roleToAdd {
			// Role already exists, no need to add it
			return nil
		}
	}

	// Add the new role to the slice
	updatedRoles := append(roles, roleToAdd)

	userProfiles[twilioRoleUserAttribute] = updatedRoles
	user.Profile = &userProfiles

	_, _, err = o.client.User.UpdateUser(ctx, userId, *user, nil)
	if err != nil {
		return fmt.Errorf("failed to update user: %v", err)
	}

	return nil
}
