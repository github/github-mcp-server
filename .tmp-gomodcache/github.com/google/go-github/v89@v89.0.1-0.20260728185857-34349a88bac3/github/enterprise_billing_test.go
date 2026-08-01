// Copyright 2026 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestEnterpriseService_GetUsageReport(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/test-enterprise/settings/billing/usage", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"year":           "2023",
			"month":          "8",
			"cost_center_id": "cc-123",
		})
		fmt.Fprint(w, `{
			"usageItems": [
				{
					"date": "2023-08-01",
					"product": "Actions",
					"sku": "Actions Linux",
					"quantity": 100,
					"unitType": "minutes",
					"pricePerUnit": 0.008,
					"grossAmount": 0.8,
					"discountAmount": 0,
					"netAmount": 0.8,
					"organizationName": "GitHub",
					"repositoryName": "github/example"
				}
			]
		}`)
	})

	ctx := t.Context()
	opts := &EnterpriseUsageReportOptions{
		Year:         2023,
		Month:        8,
		CostCenterID: "cc-123",
	}
	report, resp, err := client.Enterprise.GetUsageReport(ctx, "test-enterprise", opts)
	if err != nil {
		t.Errorf("GetUsageReport returned error: %v", err)
	}

	if resp == nil {
		t.Error("GetUsageReport returned nil response")
	}

	want := &EnterpriseUsageReport{
		UsageItems: []*EnterpriseUsageItem{
			{
				Date:             "2023-08-01",
				Product:          "Actions",
				SKU:              "Actions Linux",
				Quantity:         100.0,
				UnitType:         "minutes",
				PricePerUnit:     0.008,
				GrossAmount:      0.8,
				DiscountAmount:   0.0,
				NetAmount:        0.8,
				OrganizationName: "GitHub",
				RepositoryName:   Ptr("github/example"),
			},
		},
	}
	if !cmp.Equal(report, want) {
		t.Errorf("GetUsageReport returned %+v, want %+v", report, want)
	}

	const methodName = "GetUsageReport"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.GetUsageReport(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetUsageReport(ctx, "test-enterprise", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetUsageReport_InvalidEnterprise(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Enterprise.GetUsageReport(ctx, "%", nil)
	testURLParseError(t, err)
}

func TestEnterpriseService_GetUsageReport_APIError(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/test-enterprise/settings/billing/usage", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message": "Forbidden"}`)
	})

	ctx := t.Context()
	_, resp, err := client.Enterprise.GetUsageReport(ctx, "test-enterprise", nil)
	if err == nil {
		t.Error("GetUsageReport should return error on 403 response")
	}
	if resp == nil {
		t.Error("GetUsageReport should return response even on error")
	}
}

func TestEnterpriseService_GetUsageSummary(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/test-enterprise/settings/billing/usage/summary", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"year":    "2025",
			"product": "Actions",
		})
		fmt.Fprint(w, `{
			"timePeriod": {
				"year": 2025
			},
			"enterprise": "GitHub",
			"usageItems": [
				{
					"product": "Actions",
					"sku": "actions_linux",
					"unitType": "minutes",
					"pricePerUnit": 0.008,
					"grossQuantity": 1000,
					"grossAmount": 8,
					"discountQuantity": 0,
					"discountAmount": 0,
					"netQuantity": 1000,
					"netAmount": 8
				}
			]
		}`)
	})

	ctx := t.Context()
	opts := &EnterpriseUsageSummaryOptions{
		EnterpriseUsageReportOptions: EnterpriseUsageReportOptions{Year: 2025},
		Product:                      "Actions",
	}
	report, resp, err := client.Enterprise.GetUsageSummary(ctx, "test-enterprise", opts)
	if err != nil {
		t.Errorf("GetUsageSummary returned error: %v", err)
	}

	if resp == nil {
		t.Error("GetUsageSummary returned nil response")
	}

	want := &EnterpriseUsageSummaryReport{
		TimePeriod: EnterpriseUsageTimePeriod{
			Year: 2025,
		},
		Enterprise: "GitHub",
		UsageItems: []*EnterpriseUsageSummaryItem{
			{
				Product:          "Actions",
				SKU:              "actions_linux",
				UnitType:         "minutes",
				PricePerUnit:     0.008,
				GrossQuantity:    1000.0,
				GrossAmount:      8.0,
				DiscountQuantity: 0.0,
				DiscountAmount:   0.0,
				NetQuantity:      1000.0,
				NetAmount:        8.0,
			},
		},
	}
	if !cmp.Equal(report, want) {
		t.Errorf("GetUsageSummary returned %+v, want %+v", report, want)
	}

	const methodName = "GetUsageSummary"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.GetUsageSummary(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetUsageSummary(ctx, "test-enterprise", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetUsageSummary_InvalidEnterprise(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Enterprise.GetUsageSummary(ctx, "%", nil)
	testURLParseError(t, err)
}

func TestEnterpriseService_GetUsageSummary_APIError(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/test-enterprise/settings/billing/usage/summary", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message": "Not Found"}`)
	})

	ctx := t.Context()
	_, resp, err := client.Enterprise.GetUsageSummary(ctx, "test-enterprise", nil)
	if err == nil {
		t.Error("GetUsageSummary should return error on 404 response")
	}
	if resp == nil {
		t.Error("GetUsageSummary should return response even on error")
	}
}

func TestEnterpriseService_GetPremiumRequestUsageReport(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/test-enterprise/settings/billing/premium_request/usage", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"year":  "2025",
			"month": "10",
			"user":  "testuser",
		})
		fmt.Fprint(w, `{
			"timePeriod": {
				"year": 2025,
				"month": 10
			},
			"enterprise": "GitHub",
			"organization": "GitHub",
			"user": "testuser",
			"product": "Copilot",
			"model": "GPT-5",
			"usageItems": [
				{
					"product": "Copilot",
					"sku": "Copilot Premium Request",
					"model": "GPT-5",
					"unitType": "requests",
					"pricePerUnit": 0.04,
					"grossQuantity": 100,
					"grossAmount": 4.0,
					"discountQuantity": 0,
					"discountAmount": 0.0,
					"netQuantity": 100,
					"netAmount": 4.0
				}
			]
		}`)
	})

	ctx := t.Context()
	opts := &EnterprisePremiumRequestUsageReportOptions{
		EnterpriseUsageReportOptions: EnterpriseUsageReportOptions{Year: 2025, Month: 10},
		User:                         "testuser",
	}
	report, resp, err := client.Enterprise.GetPremiumRequestUsageReport(ctx, "test-enterprise", opts)
	if err != nil {
		t.Errorf("GetPremiumRequestUsageReport returned error: %v", err)
	}

	if resp == nil {
		t.Error("GetPremiumRequestUsageReport returned nil response")
	}

	want := &EnterpriseAggregatedUsageReport{
		TimePeriod: EnterpriseUsageTimePeriod{
			Year:  2025,
			Month: Ptr(10),
		},
		Enterprise:   "GitHub",
		Organization: Ptr("GitHub"),
		User:         Ptr("testuser"),
		Product:      Ptr("Copilot"),
		Model:        Ptr("GPT-5"),
		UsageItems: []*EnterpriseAggregatedUsageItem{
			{
				Product:          "Copilot",
				SKU:              "Copilot Premium Request",
				Model:            "GPT-5",
				UnitType:         "requests",
				PricePerUnit:     0.04,
				GrossQuantity:    100.0,
				GrossAmount:      4.0,
				DiscountQuantity: 0.0,
				DiscountAmount:   0.0,
				NetQuantity:      100.0,
				NetAmount:        4.0,
			},
		},
	}
	if !cmp.Equal(report, want) {
		t.Errorf("GetPremiumRequestUsageReport returned %+v, want %+v", report, want)
	}

	const methodName = "GetPremiumRequestUsageReport"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.GetPremiumRequestUsageReport(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetPremiumRequestUsageReport(ctx, "test-enterprise", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetPremiumRequestUsageReport_InvalidEnterprise(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Enterprise.GetPremiumRequestUsageReport(ctx, "%", nil)
	testURLParseError(t, err)
}

func TestEnterpriseService_GetPremiumRequestUsageReport_APIError(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/test-enterprise/settings/billing/premium_request/usage", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message": "Unauthorized"}`)
	})

	ctx := t.Context()
	_, resp, err := client.Enterprise.GetPremiumRequestUsageReport(ctx, "test-enterprise", nil)
	if err == nil {
		t.Error("GetPremiumRequestUsageReport should return error on 401 response")
	}
	if resp == nil {
		t.Error("GetPremiumRequestUsageReport should return response even on error")
	}
}

func TestEnterpriseService_GetAICreditUsage(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/test-enterprise/settings/billing/ai_credit/usage", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"year":  "2025",
			"month": "6",
			"user":  "testuser",
		})
		fmt.Fprint(w, `{
			"timePeriod": {
				"year": 2025,
				"month": 6
			},
			"enterprise": "GitHub",
			"organization": "GitHub",
			"user": "testuser",
			"product": "Copilot",
			"model": "GPT-5",
			"usageItems": [
				{
					"product": "Copilot",
					"sku": "Copilot AI Credits",
					"model": "GPT-5",
					"unitType": "credits",
					"pricePerUnit": 0.01,
					"grossQuantity": 100,
					"grossAmount": 1.0,
					"discountQuantity": 0,
					"discountAmount": 0.0,
					"netQuantity": 100,
					"netAmount": 1.0
				}
			]
		}`)
	})

	ctx := t.Context()
	opts := &EnterprisePremiumRequestUsageReportOptions{
		EnterpriseUsageReportOptions: EnterpriseUsageReportOptions{Year: 2025, Month: 6},
		User:                         "testuser",
	}
	report, resp, err := client.Enterprise.GetAICreditUsage(ctx, "test-enterprise", opts)
	if err != nil {
		t.Errorf("GetAICreditUsage returned error: %v", err)
	}

	if resp == nil {
		t.Error("GetAICreditUsage returned nil response")
	}

	want := &EnterpriseAggregatedUsageReport{
		TimePeriod: EnterpriseUsageTimePeriod{
			Year:  2025,
			Month: Ptr(6),
		},
		Enterprise:   "GitHub",
		Organization: Ptr("GitHub"),
		User:         Ptr("testuser"),
		Product:      Ptr("Copilot"),
		Model:        Ptr("GPT-5"),
		UsageItems: []*EnterpriseAggregatedUsageItem{
			{
				Product:          "Copilot",
				SKU:              "Copilot AI Credits",
				Model:            "GPT-5",
				UnitType:         "credits",
				PricePerUnit:     0.01,
				GrossQuantity:    100.0,
				GrossAmount:      1.0,
				DiscountQuantity: 0.0,
				DiscountAmount:   0.0,
				NetQuantity:      100.0,
				NetAmount:        1.0,
			},
		},
	}
	if !cmp.Equal(report, want) {
		t.Errorf("GetAICreditUsage returned %+v, want %+v", report, want)
	}

	const methodName = "GetAICreditUsage"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.GetAICreditUsage(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetAICreditUsage(ctx, "test-enterprise", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetAICreditUsage_InvalidEnterprise(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Enterprise.GetAICreditUsage(ctx, "%", nil)
	testURLParseError(t, err)
}

func TestEnterpriseService_GetAICreditUsage_APIError(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/test-enterprise/settings/billing/ai_credit/usage", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"message": "Bad Request"}`)
	})

	ctx := t.Context()
	_, resp, err := client.Enterprise.GetAICreditUsage(ctx, "test-enterprise", nil)
	if err == nil {
		t.Error("GetAICreditUsage should return error on 400 response")
	}
	if resp == nil {
		t.Error("GetAICreditUsage should return response even on error")
	}
}
