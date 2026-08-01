// Copyright 2025 The go-github AUTHORS. All rights reserved.
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

func TestEnterpriseService_Settings(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/manage/v1/config/settings", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"private_mode": false,
			"public_pages": false,
			"subdomain_isolation": true,
			"signup_enabled": false,
			"github_hostname": "ghe.local",
			"identicons_host": "dotcom",
			"http_proxy": null,
			"auth_mode": "default",
			"expire_sessions": false,
			"admin_password": null,
			"configuration_id": 1401777404,
			"configuration_run_count": 4,
			"avatar": {
				"enabled": false,
				"uri": ""
			},
			"customer": {
				"name": "GitHub",
				"email": "stannis",
				"uuid": "af6cac80-e4e1-012e-d822-1231380e52e9",
				"secret_key_data": "-",
				"public_key_data": "-"
			},
			"license": {
				"seats": 0,
				"evaluation": false,
				"perpetual": false,
				"unlimited_seating": true,
				"support_key": "ssh-rsa AAAAB3N....",
				"ssh_allowed": true,
				"cluster_support": false,
				"expire_at": `+referenceTimeStr+`
			},
			"github_ssl": {
				"enabled": false,
				"cert": null,
				"key": null
			},
			"ldap": {
				"host": null,
				"port": 0,
				"base": [],
				"uid": null,
				"bind_dn": null,
				"password": null,
				"method": "Plain",
				"search_strategy": "detect",
				"user_groups": [],
				"admin_group": null,
				"virtual_attribute_enabled": false,
				"recursive_group_search": false,
				"posix_support": true,
				"user_sync_emails": false,
				"user_sync_keys": false,
				"user_sync_interval": 4,
				"team_sync_interval": 4,
				"sync_enabled": false,
				"reconciliation": {
				"user": null,
				"org": null
				},
				"profile": {
				"uid": "uid",
				"name": null,
				"mail": null,
				"key": null
				}
			},
			"cas": {
				"url": null
			},
			"saml": {
				"sso_url": null,
				"certificate": null,
				"certificate_path": null,
				"issuer": null,
				"idp_initiated_sso": false,
				"disable_admin_demote": false
			},
			"github_oauth": {
				"client_id": "12313412",
				"client_secret": "kj123131132",
				"organization_name": "Homestar Runners",
				"organization_team": "homestarrunners/characters"
			},
			"smtp": {
				"enabled": true,
				"address": "smtp.example.com",
				"authentication": "plain",
				"port": "1234",
				"domain": "blah",
				"username": "foo",
				"user_name": "mr_foo",
				"enable_starttls_auto": true,
				"password": "bar",
				"discard-to-noreply-address": true,
				"support_address": "enterprise@github.com",
				"support_address_type": "email",
				"noreply_address": "noreply@github.com"
			},
			"ntp": {
				"primary_server": "example.com/primary",
				"secondary_server": "example.com/secondary"
			},
			"timezone": null,
			"snmp": {
				"enabled": false,
				"community": ""
			},
			"syslog": {
				"enabled": false,
				"server": null,
				"protocol_name": "udp"
			},
			"assets": null,
			"pages": {
				"enabled": true
			},
			"collectd": {
				"enabled": false,
				"server": null,
				"port": 0,
				"encryption": null,
				"username": null,
				"password": null
			},
			"mapping": {
				"enabled": true,
				"tileserver": null,
				"basemap": "company.map-qsz2zrvs",
				"token": null
			},
			"load_balancer": null
			}`)
	})

	ctx := t.Context()
	configSettings, _, err := client.Enterprise.Settings(ctx)
	if err != nil {
		t.Errorf("Enterprise.Settings returned error: %v", err)
	}

	want := &ConfigSettings{
		PrivateMode:           Ptr(false),
		PublicPages:           Ptr(false),
		SubdomainIsolation:    Ptr(true),
		SignupEnabled:         Ptr(false),
		GithubHostname:        Ptr("ghe.local"),
		IdenticonsHost:        Ptr("dotcom"),
		HTTPProxy:             nil,
		AuthMode:              Ptr("default"),
		ExpireSessions:        Ptr(false),
		AdminPassword:         nil,
		ConfigurationID:       Ptr(int64(1401777404)),
		ConfigurationRunCount: Ptr(4),
		Avatar: &ConfigSettingsAvatar{
			Enabled: Ptr(false),
			URI:     Ptr(""),
		},
		Customer: &ConfigSettingsCustomer{
			Name:          Ptr("GitHub"),
			Email:         Ptr("stannis"),
			UUID:          Ptr("af6cac80-e4e1-012e-d822-1231380e52e9"),
			Secret:        nil,
			PublicKeyData: Ptr("-"),
		},
		License: &ConfigSettingsLicenseSettings{
			Seats:            Ptr(0),
			Evaluation:       Ptr(false),
			Perpetual:        Ptr(false),
			UnlimitedSeating: Ptr(true),
			SupportKey:       Ptr("ssh-rsa AAAAB3N...."),
			SSHAllowed:       Ptr(true),
			ClusterSupport:   Ptr(false),
			ExpireAt:         &referenceTimestamp,
		},
		GithubSSL: &ConfigSettingsGithubSSL{
			Enabled: Ptr(false),
			Cert:    nil,
			Key:     nil,
		},
		LDAP: &ConfigSettingsLDAP{
			Host:                    nil,
			Port:                    Ptr(0),
			Base:                    []string{},
			UID:                     nil,
			BindDN:                  nil,
			Password:                nil,
			Method:                  Ptr("Plain"),
			SearchStrategy:          Ptr("detect"),
			UserGroups:              []string{},
			AdminGroup:              nil,
			VirtualAttributeEnabled: Ptr(false),
			RecursiveGroupSearch:    Ptr(false),
			PosixSupport:            Ptr(true),
			UserSyncEmails:          Ptr(false),
			UserSyncKeys:            Ptr(false),
			UserSyncInterval:        Ptr(4),
			TeamSyncInterval:        Ptr(4),
			SyncEnabled:             Ptr(false),
			Reconciliation: &ConfigSettingsLDAPReconciliation{
				User: nil,
				Org:  nil,
			},
			Profile: &ConfigSettingsLDAPProfile{
				UID:  Ptr("uid"),
				Name: nil,
				Mail: nil,
				Key:  nil,
			},
		},
		CAS: &ConfigSettingsCAS{
			URL: nil,
		},
		SAML: &ConfigSettingsSAML{
			SSOURL:             nil,
			Certificate:        nil,
			CertificatePath:    nil,
			Issuer:             nil,
			IDPInitiatedSSO:    Ptr(false),
			DisableAdminDemote: Ptr(false),
		},
		GithubOAuth: &ConfigSettingsGithubOAuth{
			ClientID:         Ptr("12313412"),
			ClientSecret:     Ptr("kj123131132"),
			OrganizationName: Ptr("Homestar Runners"),
			OrganizationTeam: Ptr("homestarrunners/characters"),
		},
		SMTP: &ConfigSettingsSMTP{
			Enabled:                 Ptr(true),
			Address:                 Ptr("smtp.example.com"),
			Authentication:          Ptr("plain"),
			Port:                    Ptr("1234"),
			Domain:                  Ptr("blah"),
			Username:                Ptr("foo"),
			UserName:                Ptr("mr_foo"),
			Password:                Ptr("bar"),
			DiscardToNoreplyAddress: Ptr(true),
			SupportAddress:          Ptr("enterprise@github.com"),
			SupportAddressType:      Ptr("email"),
			NoreplyAddress:          Ptr("noreply@github.com"),
			EnableStarttlsAuto:      Ptr(true),
		},
		NTP: &ConfigSettingsNTP{
			PrimaryServer:   Ptr("example.com/primary"),
			SecondaryServer: Ptr("example.com/secondary"),
		},
		Timezone: nil,
		SNMP: &ConfigSettingsSNMP{
			Enabled:   Ptr(false),
			Community: Ptr(""),
		},
		Syslog: &ConfigSettingsSyslog{
			Enabled:      Ptr(false),
			Server:       nil,
			ProtocolName: Ptr("udp"),
		},
		Assets: nil,
		Pages: &ConfigSettingsPagesSettings{
			Enabled: Ptr(true),
		},
		Collectd: &ConfigSettingsCollectd{
			Enabled:    Ptr(false),
			Server:     nil,
			Port:       Ptr(0),
			Encryption: nil,
			Username:   nil,
			Password:   nil,
		},
		Mapping: &ConfigSettingsMapping{
			Enabled:    Ptr(true),
			Tileserver: nil,
			Basemap:    Ptr("company.map-qsz2zrvs"),
			Token:      nil,
		},
		LoadBalancer: nil,
	}
	if diff := cmp.Diff(want, configSettings); diff != "" {
		t.Errorf("diff mismatch (-want +got):\n%v", diff)
	}

	const methodName = "Settings"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.Settings(ctx)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_NodeMetadata(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/manage/v1/config/nodes", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"uuid":          "1234-1234",
			"cluster_roles": "primary",
		})
		fmt.Fprint(w, `{
			"topology": "Cluster",
			"nodes": [{
				"hostname": "data1",
				"uuid": "1b6cf518-f97c-11ed-8544-061d81f7eedb",
				"cluster_roles": [
					"ConsulServer"
				]
			}]
		}`)
	})

	opt := &NodeQueryOptions{
		UUID: Ptr("1234-1234"), ClusterRoles: Ptr("primary"),
	}
	ctx := t.Context()
	configNodes, _, err := client.Enterprise.NodeMetadata(ctx, opt)
	if err != nil {
		t.Errorf("Enterprise.NodeMetadata returned error: %v", err)
	}

	want := &NodeMetadataStatus{
		Topology: Ptr("Cluster"),
		Nodes: []*NodeDetails{{
			Hostname: Ptr("data1"),
			UUID:     Ptr("1b6cf518-f97c-11ed-8544-061d81f7eedb"),
			ClusterRoles: []string{
				"ConsulServer",
			},
		}},
	}
	if !cmp.Equal(configNodes, want) {
		t.Errorf("Enterprise.NodeMetadata returned %+v, want %+v", configNodes, want)
	}

	const methodName = "NodeMetadata"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.NodeMetadata(ctx, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_LicenseStatus(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/manage/v1/config/license/check", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{
			"status": "valid"
			}]`)
	})

	ctx := t.Context()
	licenseCheck, _, err := client.Enterprise.LicenseStatus(ctx)
	if err != nil {
		t.Errorf("Enterprise.LicenseStatus returned error: %v", err)
	}

	want := []*LicenseCheck{{
		Status: Ptr("valid"),
	}}
	if !cmp.Equal(licenseCheck, want) {
		t.Errorf("Enterprise.LicenseStatus returned %+v, want %+v", licenseCheck, want)
	}

	const methodName = "LicenseStatus"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.LicenseStatus(ctx)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_License(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/manage/v1/config/license", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"advancedSecurityEnabled": true,
			"advancedSecuritySeats": 0,
			"clusterSupport": false,
			"company": "GitHub",
			"croquetSupport": true,
			"customTerms": true,
			"evaluation": false,
			"expireAt": `+refTimeStr(1136178000)+`,
			"insightsEnabled": true,
			"insightsExpireAt": `+refTimeStr(1136178001)+`,
			"learningLabEvaluationExpires": `+refTimeStr(1136178002)+`,
			"learningLabSeats": 100,
			"perpetual": false,
			"referenceNumber": "32a145",
			"seats": 0,
			"sshAllowed": true,
			"supportKey": true,
			"unlimitedSeating": true
		}`)
	})

	ctx := t.Context()
	license, _, err := client.Enterprise.License(ctx)
	if err != nil {
		t.Errorf("Enterprise.License returned error: %v", err)
	}

	want := &LicenseStatus{
		AdvancedSecurityEnabled:      Ptr(true),
		AdvancedSecuritySeats:        Ptr(0),
		ClusterSupport:               Ptr(false),
		Company:                      Ptr("GitHub"),
		CroquetSupport:               Ptr(true),
		CustomTerms:                  Ptr(true),
		Evaluation:                   Ptr(false),
		ExpireAt:                     refTimestamp(1136178000),
		InsightsEnabled:              Ptr(true),
		InsightsExpireAt:             refTimestamp(1136178001),
		LearningLabEvaluationExpires: refTimestamp(1136178002),
		LearningLabSeats:             Ptr(100),
		Perpetual:                    Ptr(false),
		ReferenceNumber:              Ptr("32a145"),
		Seats:                        Ptr(0),
		SSHAllowed:                   Ptr(true),
		SupportKey:                   Ptr(true),
		UnlimitedSeating:             Ptr(true),
	}
	if diff := cmp.Diff(want, license); diff != "" {
		t.Errorf("diff mismatch (-want +got):\n%v", diff)
	}
	const methodName = "License"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.License(ctx)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_ConfigApplyEvents(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/manage/v1/config/apply/events", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"nodes": [{
				"node": "ghes-01.lan",
				"last_request_id": "387cd628c06d606700e79be368e5e574:0cde553750689c76:0000000000000000",
				"events": [{
					"timestamp": `+referenceTimeStr+`,
					"severity_text": "INFO",
					"body": "Validating services",
					"event_name": "Enterprise::ConfigApply::PhaseValidation#config_phase_validation",
					"topology": "multinode",
					"hostname": "ghes-01.lan",
					"config_run_id": "d34db33f",
					"trace_id": "387cd628c06d606700e79be368e5e574",
					"span_id": "0cde553750689c76",
					"span_parent_id": 0,
					"span_depth": 0
					}]
				}]
			}`)
	})

	input := &ConfigApplyEventsOptions{
		LastRequestID: Ptr("387cd628c06d606700e79be368e5e574:0cde553750689"),
	}

	ctx := t.Context()
	configEvents, _, err := client.Enterprise.ConfigApplyEvents(ctx, input)
	if err != nil {
		t.Errorf("Enterprise.ConfigApplyEvents returned error: %v", err)
	}

	want := &ConfigApplyEvents{
		Nodes: []*ConfigApplyEventsNode{{
			Node:          Ptr("ghes-01.lan"),
			LastRequestID: Ptr("387cd628c06d606700e79be368e5e574:0cde553750689c76:0000000000000000"),
			Events: []*ConfigApplyEventsNodeEvent{{
				Timestamp:    &referenceTimestamp,
				SeverityText: Ptr("INFO"),
				Body:         Ptr("Validating services"),
				EventName:    Ptr("Enterprise::ConfigApply::PhaseValidation#config_phase_validation"),
				Topology:     Ptr("multinode"),
				Hostname:     Ptr("ghes-01.lan"),
				ConfigRunID:  Ptr("d34db33f"),
				TraceID:      Ptr("387cd628c06d606700e79be368e5e574"),
				SpanID:       Ptr("0cde553750689c76"),
				SpanParentID: Ptr(int64(0)),
				SpanDepth:    Ptr(0),
			}},
		}},
	}
	if diff := cmp.Diff(want, configEvents); diff != "" {
		t.Errorf("diff mismatch (-want +got):\n%v", diff)
	}
	if !cmp.Equal(configEvents, want) {
		t.Errorf("Enterprise.ConfigApplyEvents returned %+v, want %+v", configEvents, want)
	}

	const methodName = "ConfigApplyEvents"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ConfigApplyEvents(ctx, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_UpdateSettings(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/manage/v1/config/settings", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")

		w.WriteHeader(http.StatusNoContent)
	})

	input := &ConfigSettings{
		PrivateMode: Ptr(false),
	}

	ctx := t.Context()
	if _, err := client.Enterprise.UpdateSettings(ctx, input); err != nil {
		t.Errorf("Enterprise.UpdateSettings returned error: %v", err)
	}

	const methodName = "UpdateSettings"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Enterprise.UpdateSettings(ctx, nil)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Enterprise.UpdateSettings(ctx, input)
	})
}

func TestEnterpriseService_UploadLicense(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/manage/v1/config/license", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Enterprise.UploadLicense(ctx, "abc"); err != nil {
		t.Errorf("Enterprise.UploadLicense returned error: %v", err)
	}

	const methodName = "UploadLicense"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Enterprise.UploadLicense(ctx, "")
	})
}

func TestEnterpriseService_InitialConfig(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &InitialConfigOptions{
		License:  "1234-1234",
		Password: "password",
	}

	mux.HandleFunc("/manage/v1/config/init", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
	})

	ctx := t.Context()
	if _, err := client.Enterprise.InitialConfig(ctx, "1234-1234", "password"); err != nil {
		t.Errorf("Enterprise.InitialConfig returned error: %v", err)
	}

	const methodName = "InitialConfig"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Enterprise.InitialConfig(ctx, "", "")
	})
}

func TestEnterpriseService_ConfigApply(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &ConfigApplyOptions{
		RunID: Ptr("1234"),
	}

	mux.HandleFunc("/manage/v1/config/apply", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{ "run_id": "1234" }`)
	})

	ctx := t.Context()
	configApply, _, err := client.Enterprise.ConfigApply(ctx, input)
	if err != nil {
		t.Errorf("Enterprise.ConfigApply returned error: %v", err)
	}
	want := &ConfigApplyOptions{
		RunID: Ptr("1234"),
	}
	if !cmp.Equal(configApply, want) {
		t.Errorf("Enterprise.ConfigApply returned %+v, want %+v", configApply, want)
	}
	const methodName = "ConfigApply"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ConfigApply(ctx, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_ConfigApplyStatus(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &ConfigApplyOptions{
		RunID: Ptr("1234"),
	}

	mux.HandleFunc("/manage/v1/config/apply", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{
			"running": true,
			"successful": false,
			"nodes": [
				{
				"run_id": "d34db33f",
				"hostname": "ghes-01.lan",
				"running": true,
				"successful": false
				}
			]
		}`)
	})

	ctx := t.Context()
	configApplyStatus, _, err := client.Enterprise.ConfigApplyStatus(ctx, input)
	if err != nil {
		t.Errorf("Enterprise.ConfigApplyStatus returned error: %v", err)
	}
	want := &ConfigApplyStatus{
		Running:    Ptr(true),
		Successful: Ptr(false),
		Nodes: []*ConfigApplyStatusNode{{
			RunID:      Ptr("d34db33f"),
			Hostname:   Ptr("ghes-01.lan"),
			Running:    Ptr(true),
			Successful: Ptr(false),
		}},
	}
	if !cmp.Equal(configApplyStatus, want) {
		t.Errorf("Enterprise.ConfigApplyStatus returned %+v, want %+v", configApplyStatus, want)
	}
	const methodName = "ConfigApplyStatus"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ConfigApplyStatus(ctx, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
