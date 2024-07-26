package main

import (
	"context"
	"fmt"

	"github.com/okta/okta-sdk-golang/v2/okta"
)

type oktaClient struct {
	client *okta.Client
}

func newOktaClient(oktaDomain, apiToken string) (*oktaClient, error) {
	ctx := context.TODO()

	ctx, client, err := okta.NewClient(ctx, okta.WithOrgUrl(oktaDomain), okta.WithToken(apiToken))
	if err != nil {
		return nil, fmt.Errorf("failed to create Okta client: %v", err)
	}

	return &oktaClient{client: client}, nil
}
