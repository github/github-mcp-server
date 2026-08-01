// Copyright 2026 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// EnterpriseUsageReportOptions specifies optional parameters for the EnterpriseService.GetUsageReport method.
type EnterpriseUsageReportOptions struct {
	Year         int    `url:"year,omitempty"`
	Month        int    `url:"month,omitempty"`
	Day          int    `url:"day,omitempty"`
	CostCenterID string `url:"cost_center_id,omitempty"`
}

// EnterprisePremiumRequestUsageReportOptions specifies optional parameters for the EnterpriseService.GetPremiumRequestUsageReport and EnterpriseService.GetAICreditUsage methods.
type EnterprisePremiumRequestUsageReportOptions struct {
	EnterpriseUsageReportOptions
	Organization string `url:"organization,omitempty"`
	User         string `url:"user,omitempty"`
	Model        string `url:"model,omitempty"`
	Product      string `url:"product,omitempty"`
}

// EnterpriseUsageSummaryOptions specifies optional parameters for the EnterpriseService.GetUsageSummary method.
type EnterpriseUsageSummaryOptions struct {
	EnterpriseUsageReportOptions
	Organization string `url:"organization,omitempty"`
	Repository   string `url:"repository,omitempty"`
	Product      string `url:"product,omitempty"`
	SKU          string `url:"sku,omitempty"`
}

// EnterpriseUsageTimePeriod represents a time period used in aggregated enterprise billing reports.
type EnterpriseUsageTimePeriod struct {
	Year  int  `json:"year"`
	Month *int `json:"month,omitempty"`
	Day   *int `json:"day,omitempty"`
}

// EnterpriseAggregatedUsageItem represents a single usage line item in an enterprise billing premium request or AI credit report.
type EnterpriseAggregatedUsageItem struct {
	Product          string  `json:"product"`
	SKU              string  `json:"sku"`
	Model            string  `json:"model"`
	UnitType         string  `json:"unitType"`
	PricePerUnit     float64 `json:"pricePerUnit"`
	GrossQuantity    float64 `json:"grossQuantity"`
	GrossAmount      float64 `json:"grossAmount"`
	DiscountQuantity float64 `json:"discountQuantity"`
	DiscountAmount   float64 `json:"discountAmount"`
	NetQuantity      float64 `json:"netQuantity"`
	NetAmount        float64 `json:"netAmount"`
}

// EnterpriseAggregatedUsageReport represents the aggregated billing usage report response for the premium request and AI credit endpoints.
type EnterpriseAggregatedUsageReport struct {
	TimePeriod   EnterpriseUsageTimePeriod        `json:"timePeriod"`
	Enterprise   string                           `json:"enterprise"`
	Organization *string                          `json:"organization,omitempty"`
	User         *string                          `json:"user,omitempty"`
	Product      *string                          `json:"product,omitempty"`
	Model        *string                          `json:"model,omitempty"`
	CostCenter   *BillingCostCenter               `json:"costCenter,omitempty"`
	UsageItems   []*EnterpriseAggregatedUsageItem `json:"usageItems"`
}

// EnterpriseUsageSummaryItem represents a single usage line item in an enterprise billing usage summary report.
type EnterpriseUsageSummaryItem struct {
	Product          string  `json:"product"`
	SKU              string  `json:"sku"`
	UnitType         string  `json:"unitType"`
	PricePerUnit     float64 `json:"pricePerUnit"`
	GrossQuantity    float64 `json:"grossQuantity"`
	GrossAmount      float64 `json:"grossAmount"`
	DiscountQuantity float64 `json:"discountQuantity"`
	DiscountAmount   float64 `json:"discountAmount"`
	NetQuantity      float64 `json:"netQuantity"`
	NetAmount        float64 `json:"netAmount"`
}

// EnterpriseUsageSummaryReport represents the billing usage summary report response for the EnterpriseService.GetUsageSummary endpoint.
type EnterpriseUsageSummaryReport struct {
	TimePeriod   EnterpriseUsageTimePeriod     `json:"timePeriod"`
	Enterprise   string                        `json:"enterprise"`
	Organization *string                       `json:"organization,omitempty"`
	Repository   *string                       `json:"repository,omitempty"`
	Product      *string                       `json:"product,omitempty"`
	SKU          *string                       `json:"sku,omitempty"`
	CostCenter   *BillingCostCenter            `json:"costCenter,omitempty"`
	UsageItems   []*EnterpriseUsageSummaryItem `json:"usageItems"`
}

// BillingCostCenter represents a cost center reference embedded in enterprise billing usage reports.
type BillingCostCenter struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// EnterpriseUsageItem represents a single usage line item in an enterprise billing platform report.
type EnterpriseUsageItem struct {
	Date             string  `json:"date"`
	Product          string  `json:"product"`
	SKU              string  `json:"sku"`
	Quantity         float64 `json:"quantity"`
	UnitType         string  `json:"unitType"`
	PricePerUnit     float64 `json:"pricePerUnit"`
	GrossAmount      float64 `json:"grossAmount"`
	DiscountAmount   float64 `json:"discountAmount"`
	NetAmount        float64 `json:"netAmount"`
	OrganizationName string  `json:"organizationName"`
	RepositoryName   *string `json:"repositoryName,omitempty"`
}

// EnterpriseUsageReport represents the enterprise billing usage report response.
type EnterpriseUsageReport struct {
	UsageItems []*EnterpriseUsageItem `json:"usageItems,omitempty"`
}

// GetUsageReport returns a report of the total usage for an enterprise using the enhanced billing platform.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/billing/usage?apiVersion=2022-11-28#get-billing-usage-report-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/settings/billing/usage
func (s *EnterpriseService) GetUsageReport(ctx context.Context, enterprise string, opts *EnterpriseUsageReportOptions) (*EnterpriseUsageReport, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/settings/billing/usage", enterprise)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var report *EnterpriseUsageReport
	resp, err := s.client.Do(req, &report)
	if err != nil {
		return nil, resp, err
	}

	return report, resp, nil
}

// GetUsageSummary returns a summary report of usage for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/billing/usage?apiVersion=2022-11-28#get-billing-usage-summary-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/settings/billing/usage/summary
func (s *EnterpriseService) GetUsageSummary(ctx context.Context, enterprise string, opts *EnterpriseUsageSummaryOptions) (*EnterpriseUsageSummaryReport, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/settings/billing/usage/summary", enterprise)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var report *EnterpriseUsageSummaryReport
	resp, err := s.client.Do(req, &report)
	if err != nil {
		return nil, resp, err
	}

	return report, resp, nil
}

// GetPremiumRequestUsageReport returns a report of the premium request usage for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/billing/usage?apiVersion=2022-11-28#get-billing-premium-request-usage-report-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/settings/billing/premium_request/usage
func (s *EnterpriseService) GetPremiumRequestUsageReport(ctx context.Context, enterprise string, opts *EnterprisePremiumRequestUsageReportOptions) (*EnterpriseAggregatedUsageReport, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/settings/billing/premium_request/usage", enterprise)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var report *EnterpriseAggregatedUsageReport
	resp, err := s.client.Do(req, &report)
	if err != nil {
		return nil, resp, err
	}

	return report, resp, nil
}

// GetAICreditUsage returns a report of the AI credit usage for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/billing/usage?apiVersion=2022-11-28#get-billing-ai-credit-usage-report-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/settings/billing/ai_credit/usage
func (s *EnterpriseService) GetAICreditUsage(ctx context.Context, enterprise string, opts *EnterprisePremiumRequestUsageReportOptions) (*EnterpriseAggregatedUsageReport, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/settings/billing/ai_credit/usage", enterprise)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var report *EnterpriseAggregatedUsageReport
	resp, err := s.client.Do(req, &report)
	if err != nil {
		return nil, resp, err
	}

	return report, resp, nil
}
