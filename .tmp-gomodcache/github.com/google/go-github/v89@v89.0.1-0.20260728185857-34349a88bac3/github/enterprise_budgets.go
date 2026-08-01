// Copyright 2026 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// BudgetScope constants represent the scope of the budget.
const (
	BudgetScopeEnterprise   = "enterprise"
	BudgetScopeOrganization = "organization"
	BudgetScopeRepository   = "repository"
	BudgetScopeCostCenter   = "cost_center"
)

// BudgetType constants represent the type of pricing for the budget.
const (
	BudgetTypeProductPricing = "ProductPricing"
	BudgetTypeSkuPricing     = "SkuPricing"
)

// EnterpriseBudgetAlerting represents alerting settings for a GitHub enterprise budget.
type EnterpriseBudgetAlerting struct {
	WillAlert       *bool    `json:"will_alert,omitempty"`
	AlertRecipients []string `json:"alert_recipients,omitempty"`
}

// EnterpriseBudget represents a GitHub enterprise budget.
type EnterpriseBudget struct {
	ID                  *string                   `json:"id,omitempty"`
	BudgetType          *string                   `json:"budget_type,omitempty"`
	BudgetProductSKU    *string                   `json:"budget_product_sku,omitempty"`
	BudgetScope         *string                   `json:"budget_scope,omitempty"`
	BudgetEntityName    *string                   `json:"budget_entity_name,omitempty"`
	BudgetAmount        *int                      `json:"budget_amount,omitempty"`
	PreventFurtherUsage *bool                     `json:"prevent_further_usage,omitempty"`
	BudgetAlerting      *EnterpriseBudgetAlerting `json:"budget_alerting,omitempty"`
}

func (b EnterpriseBudget) String() string {
	return Stringify(b)
}

// EnterpriseListBudgets represents a collection of GitHub enterprise budgets.
type EnterpriseListBudgets struct {
	Budgets     []*EnterpriseBudget `json:"budgets"`
	HasNextPage *bool               `json:"has_next_page,omitempty"`
	TotalCount  *int                `json:"total_count,omitempty"`
}

// EnterpriseCreateBudget represents the payload to create a GitHub enterprise budget.
type EnterpriseCreateBudget struct {
	BudgetAmount        int                       `json:"budget_amount"`
	PreventFurtherUsage bool                      `json:"prevent_further_usage"`
	BudgetAlerting      *EnterpriseBudgetAlerting `json:"budget_alerting"`
	BudgetScope         string                    `json:"budget_scope"`
	BudgetEntityName    *string                   `json:"budget_entity_name,omitempty"`
	BudgetType          string                    `json:"budget_type"`
	BudgetProductSKU    *string                   `json:"budget_product_sku,omitempty"`
}

// EnterpriseUpdateBudget represents the payload to update a GitHub enterprise budget.
type EnterpriseUpdateBudget struct {
	BudgetAmount        *int                      `json:"budget_amount,omitempty"`
	PreventFurtherUsage *bool                     `json:"prevent_further_usage,omitempty"`
	BudgetAlerting      *EnterpriseBudgetAlerting `json:"budget_alerting,omitempty"`
	BudgetScope         *string                   `json:"budget_scope,omitempty"`
	BudgetEntityName    *string                   `json:"budget_entity_name,omitempty"`
	BudgetType          *string                   `json:"budget_type,omitempty"`
	BudgetProductSKU    *string                   `json:"budget_product_sku,omitempty"`
}

// EnterpriseCreateOrUpdateBudgetResponse represents the response when creating or updating a budget.
type EnterpriseCreateOrUpdateBudgetResponse struct {
	Message string            `json:"message"`
	Budget  *EnterpriseBudget `json:"budget"`
}

// EnterpriseDeleteBudgetResponse represents the response when deleting a budget.
type EnterpriseDeleteBudgetResponse struct {
	Message string `json:"message"`
	ID      string `json:"id"`
}

// ListBudgets gets all budgets for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/billing/budgets?apiVersion=2022-11-28#get-all-budgets
//
//meta:operation GET /enterprises/{enterprise}/settings/billing/budgets
func (s *EnterpriseService) ListBudgets(ctx context.Context, enterprise string) (*EnterpriseListBudgets, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/settings/billing/budgets", enterprise)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var budgets *EnterpriseListBudgets
	resp, err := s.client.Do(req, &budgets)
	if err != nil {
		return nil, resp, err
	}

	return budgets, resp, nil
}

// CreateBudget creates a new budget for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/billing/budgets?apiVersion=2022-11-28#create-a-budget
//
//meta:operation POST /enterprises/{enterprise}/settings/billing/budgets
func (s *EnterpriseService) CreateBudget(ctx context.Context, enterprise string, body EnterpriseCreateBudget) (*EnterpriseCreateOrUpdateBudgetResponse, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/settings/billing/budgets", enterprise)

	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var createBudgetResponse *EnterpriseCreateOrUpdateBudgetResponse
	resp, err := s.client.Do(req, &createBudgetResponse)
	if err != nil {
		return nil, resp, err
	}

	return createBudgetResponse, resp, nil
}

// GetBudget gets a budget by ID for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/billing/budgets?apiVersion=2022-11-28#get-a-budget-by-id
//
//meta:operation GET /enterprises/{enterprise}/settings/billing/budgets/{budget_id}
func (s *EnterpriseService) GetBudget(ctx context.Context, enterprise, budgetID string) (*EnterpriseBudget, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/settings/billing/budgets/%v", enterprise, budgetID)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var budget *EnterpriseBudget
	resp, err := s.client.Do(req, &budget)
	if err != nil {
		return nil, resp, err
	}

	return budget, resp, nil
}

// UpdateBudget updates an existing budget for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/billing/budgets?apiVersion=2022-11-28#update-a-budget
//
//meta:operation PATCH /enterprises/{enterprise}/settings/billing/budgets/{budget_id}
func (s *EnterpriseService) UpdateBudget(ctx context.Context, enterprise, budgetID string, body EnterpriseUpdateBudget) (*EnterpriseCreateOrUpdateBudgetResponse, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/settings/billing/budgets/%v", enterprise, budgetID)

	req, err := s.client.NewRequest(ctx, "PATCH", u, body)
	if err != nil {
		return nil, nil, err
	}

	var updateBudgetResponse *EnterpriseCreateOrUpdateBudgetResponse
	resp, err := s.client.Do(req, &updateBudgetResponse)
	if err != nil {
		return nil, resp, err
	}

	return updateBudgetResponse, resp, nil
}

// DeleteBudget deletes a budget by ID for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/billing/budgets?apiVersion=2022-11-28#delete-a-budget
//
//meta:operation DELETE /enterprises/{enterprise}/settings/billing/budgets/{budget_id}
func (s *EnterpriseService) DeleteBudget(ctx context.Context, enterprise, budgetID string) (*EnterpriseDeleteBudgetResponse, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/settings/billing/budgets/%v", enterprise, budgetID)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var deleteBudgetResponse *EnterpriseDeleteBudgetResponse
	resp, err := s.client.Do(req, &deleteBudgetResponse)
	if err != nil {
		return nil, resp, err
	}

	return deleteBudgetResponse, resp, nil
}
