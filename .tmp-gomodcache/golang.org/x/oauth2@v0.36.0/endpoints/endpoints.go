// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package endpoints provides constants for using OAuth2 to access various services.
package endpoints

import (
	"net/url"

	"golang.org/x/oauth2"
)

// Amazon is the endpoint for Amazon.
var Amazon = oauth2.Endpoint{
	AuthURL:  "https://www.amazon.com/ap/oa",
	TokenURL: "https://api.amazon.com/auth/o2/token",
}

// Apple is the endpoint for "Sign in with Apple".
//
// Documentation: https://developer.apple.com/documentation/signinwithapplerestapi
var Apple = oauth2.Endpoint{
	AuthURL:  "https://appleid.apple.com/auth/authorize",
	TokenURL: "https://appleid.apple.com/auth/token",
}

// Asana is the endpoint for Asana.
//
// Documentation: https://developers.asana.com/docs/oauth
var Asana = oauth2.Endpoint{
	AuthURL:  "https://app.asana.com/-/oauth_authorize",
	TokenURL: "https://app.asana.com/-/oauth_token",
}

// Badgr is the endpoint for Canvas Badges.
//
// Documentation: https://community.canvaslms.com/t5/Canvas-Badges-Credentials/Developers-Build-an-app-that-integrates-with-the-Canvas-Badges/ta-p/528727
var Badgr = oauth2.Endpoint{
	AuthURL:  "https://badgr.com/auth/oauth2/authorize",
	TokenURL: "https://api.badgr.io/o/token",
}

// Battlenet is the endpoint for Battlenet.
var Battlenet = oauth2.Endpoint{
	AuthURL:  "https://battle.net/oauth/authorize",
	TokenURL: "https://battle.net/oauth/token",
}

// Bitbucket is the endpoint for Bitbucket.
var Bitbucket = oauth2.Endpoint{
	AuthURL:  "https://bitbucket.org/site/oauth2/authorize",
	TokenURL: "https://bitbucket.org/site/oauth2/access_token",
}

// Cern is the endpoint for CERN.
var Cern = oauth2.Endpoint{
	AuthURL:  "https://oauth.web.cern.ch/OAuth/Authorize",
	TokenURL: "https://oauth.web.cern.ch/OAuth/Token",
}

// Coinbase is the endpoint for Coinbase.
//
// Documentation: https://docs.cdp.coinbase.com/coinbase-app/docs/coinbase-app-reference
var Coinbase = oauth2.Endpoint{
	AuthURL:  "https://login.coinbase.com/oauth2/auth",
	TokenURL: "https://login.coinbase.com/oauth2/token",
}

// Discord is the endpoint for Discord.
//
// Documentation: https://discord.com/developers/docs/topics/oauth2#shared-resources-oauth2-urls
var Discord = oauth2.Endpoint{
	AuthURL:  "https://discord.com/oauth2/authorize",
	TokenURL: "https://discord.com/api/oauth2/token",
}

// Dropbox is the endpoint for Dropbox.
//
// Documentation: https://developers.dropbox.com/oauth-guide
var Dropbox = oauth2.Endpoint{
	AuthURL:  "https://www.dropbox.com/oauth2/authorize",
	TokenURL: "https://api.dropboxapi.com/oauth2/token",
}

// Endpoint is Ebay's OAuth 2.0 endpoint.
//
// Documentation: https://developer.ebay.com/api-docs/static/authorization_guide_landing.html
var Endpoint = oauth2.Endpoint{
	AuthURL:  "https://auth.ebay.com/oauth2/authorize",
	TokenURL: "https://api.ebay.com/identity/v1/oauth2/token",
}

// Facebook is the endpoint for Facebook.
//
// Documentation: https://developers.facebook.com/docs/facebook-login/guides/advanced/manual-flow
var Facebook = oauth2.Endpoint{
	AuthURL:  "https://www.facebook.com/v22.0/dialog/oauth",
	TokenURL: "https://graph.facebook.com/v22.0/oauth/access_token",
}

// Foursquare is the endpoint for Foursquare.
var Foursquare = oauth2.Endpoint{
	AuthURL:  "https://foursquare.com/oauth2/authorize",
	TokenURL: "https://foursquare.com/oauth2/access_token",
}

// Fitbit is the endpoint for Fitbit.
var Fitbit = oauth2.Endpoint{
	AuthURL:  "https://www.fitbit.com/oauth2/authorize",
	TokenURL: "https://api.fitbit.com/oauth2/token",
}

// GitHub is the endpoint for Github.
var GitHub = oauth2.Endpoint{
	AuthURL:       "https://github.com/login/oauth/authorize",
	TokenURL:      "https://github.com/login/oauth/access_token",
	DeviceAuthURL: "https://github.com/login/device/code",
}

// GitLab is the endpoint for GitLab.
var GitLab = oauth2.Endpoint{
	AuthURL:       "https://gitlab.com/oauth/authorize",
	TokenURL:      "https://gitlab.com/oauth/token",
	DeviceAuthURL: "https://gitlab.com/oauth/authorize_device",
}

// Google is the endpoint for Google.
var Google = oauth2.Endpoint{
	AuthURL:       "https://accounts.google.com/o/oauth2/auth",
	TokenURL:      "https://oauth2.googleapis.com/token",
	DeviceAuthURL: "https://oauth2.googleapis.com/device/code",
}

// Heroku is the endpoint for Heroku.
var Heroku = oauth2.Endpoint{
	AuthURL:  "https://id.heroku.com/oauth/authorize",
	TokenURL: "https://id.heroku.com/oauth/token",
}

// HipChat is the endpoint for HipChat.
var HipChat = oauth2.Endpoint{
	AuthURL:  "https://www.hipchat.com/users/authorize",
	TokenURL: "https://api.hipchat.com/v2/oauth/token",
}

// Instagram is the endpoint for Instagram.
var Instagram = oauth2.Endpoint{
	AuthURL:  "https://api.instagram.com/oauth/authorize",
	TokenURL: "https://api.instagram.com/oauth/access_token",
}

// KaKao is the endpoint for KaKao.
var KaKao = oauth2.Endpoint{
	AuthURL:  "https://kauth.kakao.com/oauth/authorize",
	TokenURL: "https://kauth.kakao.com/oauth/token",
}

// Line is the endpoint for Line.
//
// Documentation: https://developers.line.biz/en/docs/line-login/integrate-line-login/
var Line = oauth2.Endpoint{
	AuthURL:  "https://access.line.me/oauth2/v2.1/authorize",
	TokenURL: "https://api.line.me/oauth2/v2.1/token",
}

// LinkedIn is the endpoint for LinkedIn.
var LinkedIn = oauth2.Endpoint{
	AuthURL:  "https://www.linkedin.com/oauth/v2/authorization",
	TokenURL: "https://www.linkedin.com/oauth/v2/accessToken",
}

// Mailchimp is the endpoint for Mailchimp.
var Mailchimp = oauth2.Endpoint{
	AuthURL:  "https://login.mailchimp.com/oauth2/authorize",
	TokenURL: "https://login.mailchimp.com/oauth2/token",
}

// Mailru is the endpoint for Mail.Ru.
var Mailru = oauth2.Endpoint{
	AuthURL:  "https://o2.mail.ru/login",
	TokenURL: "https://o2.mail.ru/token",
}

// MediaMath is the endpoint for MediaMath.
var MediaMath = oauth2.Endpoint{
	AuthURL:  "https://api.mediamath.com/oauth2/v1.0/authorize",
	TokenURL: "https://api.mediamath.com/oauth2/v1.0/token",
}

// MediaMathSandbox is the endpoint for MediaMath Sandbox.
var MediaMathSandbox = oauth2.Endpoint{
	AuthURL:  "https://t1sandbox.mediamath.com/oauth2/v1.0/authorize",
	TokenURL: "https://t1sandbox.mediamath.com/oauth2/v1.0/token",
}

// Microsoft is the endpoint for Microsoft.
var Microsoft = oauth2.Endpoint{
	AuthURL:  "https://login.live.com/oauth20_authorize.srf",
	TokenURL: "https://login.live.com/oauth20_token.srf",
}

// Naver is the endpoint for Naver.
//
// Documentation: https://developers.naver.com/docs/login/devguide/devguide.md
var Naver = oauth2.Endpoint{
	AuthURL:  "https://nid.naver.com/oauth2.0/authorize",
	TokenURL: "https://nid.naver.com/oauth2.0/token",
}

// NokiaHealth is the endpoint for Nokia Health.
//
// Deprecated: Nokia Health is now Withings.
var NokiaHealth = oauth2.Endpoint{
	AuthURL:  "https://account.health.nokia.com/oauth2_user/authorize2",
	TokenURL: "https://account.health.nokia.com/oauth2/token",
}

// Odnoklassniki is the endpoint for Odnoklassniki.
var Odnoklassniki = oauth2.Endpoint{
	AuthURL:  "https://www.odnoklassniki.ru/oauth/authorize",
	TokenURL: "https://api.odnoklassniki.ru/oauth/token.do",
}

// OpenStreetMap is the endpoint for OpenStreetMap.org.
//
// Documentation: https://wiki.openstreetmap.org/wiki/OAuth
var OpenStreetMap = oauth2.Endpoint{
	AuthURL:  "https://www.openstreetmap.org/oauth2/authorize",
	TokenURL: "https://www.openstreetmap.org/oauth2/token",
}

// Patreon is the endpoint for Patreon.
var Patreon = oauth2.Endpoint{
	AuthURL:  "https://www.patreon.com/oauth2/authorize",
	TokenURL: "https://www.patreon.com/api/oauth2/token",
}

// PayPal is the endpoint for PayPal.
var PayPal = oauth2.Endpoint{
	AuthURL:  "https://www.paypal.com/webapps/auth/protocol/openidconnect/v1/authorize",
	TokenURL: "https://api.paypal.com/v1/identity/openidconnect/tokenservice",
}

// PayPalSandbox is the endpoint for PayPal Sandbox.
var PayPalSandbox = oauth2.Endpoint{
	AuthURL:  "https://www.sandbox.paypal.com/webapps/auth/protocol/openidconnect/v1/authorize",
	TokenURL: "https://api.sandbox.paypal.com/v1/identity/openidconnect/tokenservice",
}

// Pinterest is the endpoint for Pinterest.
//
// Documentation: https://developers.pinterest.com/docs/getting-started/set-up-authentication-and-authorization/
var Pinterest = oauth2.Endpoint{
	AuthURL:  "https://www.pinterest.com/oauth",
	TokenURL: "https://api.pinterest.com/v5/oauth/token",
}

// Pipedrive is the endpoint for Pipedrive.
//
// Documentation: https://developers.pipedrive.com/docs/api/v1/Oauth
var Pipedrive = oauth2.Endpoint{
	AuthURL:  "https://oauth.pipedrive.com/oauth/authorize",
	TokenURL: "https://oauth.pipedrive.com/oauth/token",
}

// QQ is the endpoint for QQ.
//
// Documentation: https://wiki.connect.qq.com/%e5%bc%80%e5%8f%91%e6%94%bb%e7%95%a5_server-side
var QQ = oauth2.Endpoint{
	AuthURL:  "https://graph.qq.com/oauth2.0/authorize",
	TokenURL: "https://graph.qq.com/oauth2.0/token",
}

// Rakuten is the endpoint for Rakuten.
//
// Documentation: https://webservice.rakuten.co.jp/documentation
var Rakuten = oauth2.Endpoint{
	AuthURL:  "https://app.rakuten.co.jp/services/authorize",
	TokenURL: "https://app.rakuten.co.jp/services/token",
}

// Slack is the endpoint for Slack.
//
// Documentation: https://api.slack.com/authentication/oauth-v2
var Slack = oauth2.Endpoint{
	AuthURL:  "https://slack.com/oauth/v2/authorize",
	TokenURL: "https://slack.com/api/oauth.v2.access",
}

// Splitwise is the endpoint for Splitwise.
//
// Documentation: https://dev.splitwise.com/
var Splitwise = oauth2.Endpoint{
	AuthURL:  "https://www.splitwise.com/oauth/authorize",
	TokenURL: "https://www.splitwise.com/oauth/token",
}

// Spotify is the endpoint for Spotify.
var Spotify = oauth2.Endpoint{
	AuthURL:  "https://accounts.spotify.com/authorize",
	TokenURL: "https://accounts.spotify.com/api/token",
}

// StackOverflow is the endpoint for Stack Overflow.
var StackOverflow = oauth2.Endpoint{
	AuthURL:  "https://stackoverflow.com/oauth",
	TokenURL: "https://stackoverflow.com/oauth/access_token",
}

// Strava is the endpoint for Strava.
var Strava = oauth2.Endpoint{
	AuthURL:  "https://www.strava.com/oauth/authorize",
	TokenURL: "https://www.strava.com/oauth/token",
}

// Twitch is the endpoint for Twitch.
var Twitch = oauth2.Endpoint{
	AuthURL:  "https://id.twitch.tv/oauth2/authorize",
	TokenURL: "https://id.twitch.tv/oauth2/token",
}

// Uber is the endpoint for Uber.
var Uber = oauth2.Endpoint{
	AuthURL:  "https://login.uber.com/oauth/v2/authorize",
	TokenURL: "https://login.uber.com/oauth/v2/token",
}

// Vk is the endpoint for Vk.
var Vk = oauth2.Endpoint{
	AuthURL:  "https://oauth.vk.com/authorize",
	TokenURL: "https://oauth.vk.com/access_token",
}

// Withings is the endpoint for Withings.
//
// Documentation: https://account.withings.com/oauth2_user/authorize2
var Withings = oauth2.Endpoint{
	AuthURL:  "https://account.withings.com/oauth2_user/authorize2",
	TokenURL: "https://account.withings.com/oauth2/token",
}

// X is the endpoint for X (Twitter).
//
// Documentation: https://docs.x.com/resources/fundamentals/authentication/oauth-2-0/user-access-token
var X = oauth2.Endpoint{
	AuthURL:  "https://x.com/i/oauth2/authorize",
	TokenURL: "https://api.x.com/2/oauth2/token",
}

// Yahoo is the endpoint for Yahoo.
var Yahoo = oauth2.Endpoint{
	AuthURL:  "https://api.login.yahoo.com/oauth2/request_auth",
	TokenURL: "https://api.login.yahoo.com/oauth2/get_token",
}

// Yandex is the endpoint for Yandex.
var Yandex = oauth2.Endpoint{
	AuthURL:  "https://oauth.yandex.com/authorize",
	TokenURL: "https://oauth.yandex.com/token",
}

// Zoom is the endpoint for Zoom.
var Zoom = oauth2.Endpoint{
	AuthURL:  "https://zoom.us/oauth/authorize",
	TokenURL: "https://zoom.us/oauth/token",
}

// Asgardeo returns a new oauth2.Endpoint for the given tenant.
//
// Documentation: https://wso2.com/asgardeo/docs/guides/authentication/oidc/discover-oidc-configs/
func AsgardeoEndpoint(tenant string) oauth2.Endpoint {
	u := url.URL{
		Scheme: "https",
		Host:   "api.asgardeo.io",
	}
	return oauth2.Endpoint{
		AuthURL:  u.JoinPath("t", tenant, "/oauth2/authorize").String(),
		TokenURL: u.JoinPath("t", tenant, "/oauth2/token").String(),
	}
}

// AzureAD returns a new oauth2.Endpoint for the given tenant at Azure Active Directory.
// If tenant is empty, it uses the tenant called `common`.
//
// For more information see:
// https://docs.microsoft.com/en-us/azure/active-directory/develop/active-directory-v2-protocols#endpoints
func AzureAD(tenant string) oauth2.Endpoint {
	if tenant == "" {
		tenant = "common"
	}
	u := url.URL{
		Scheme: "https",
		Host:   "login.microsoftonline.com",
	}
	return oauth2.Endpoint{
		AuthURL:       u.JoinPath(tenant, "/oauth2/v2.0/authorize").String(),
		TokenURL:      u.JoinPath(tenant, "/oauth2/v2.0/token").String(),
		DeviceAuthURL: u.JoinPath(tenant, "/oauth2/v2.0/devicecode").String(),
	}
}

// AzureADB2CEndpoint returns a new oauth2.Endpoint for the given tenant and policy at Azure Active Directory B2C.
// policy is the Azure B2C User flow name Example: `B2C_1_SignUpSignIn`.
//
// Documentation: https://docs.microsoft.com/en-us/azure/active-directory-b2c/tokens-overview#endpoints
func AzureADB2CEndpoint(tenant string, policy string) oauth2.Endpoint {
	u := url.URL{
		Scheme: "https",
		Host:   tenant + ".b2clogin.com",
	}
	return oauth2.Endpoint{
		AuthURL:  u.JoinPath(tenant+".onmicrosoft.com", policy, "/oauth2/v2.0/authorize").String(),
		TokenURL: u.JoinPath(tenant+".onmicrosoft.com", policy, "/oauth2/v2.0/token").String(),
	}
}

// AWSCognito returns a new oauth2.Endpoint for the supplied AWS Cognito domain which is
// linked to your Cognito User Pool.
//
// Example domain: https://testing.auth.us-east-1.amazoncognito.com
//
// For more information see:
// https://docs.aws.amazon.com/cognito/latest/developerguide/cognito-user-pools-assign-domain.html
// https://docs.aws.amazon.com/cognito/latest/developerguide/cognito-userpools-server-contract-reference.html
func AWSCognito(domain string) oauth2.Endpoint {
	u, err := url.Parse(domain)
	if err != nil || u.Scheme == "" || u.Host == "" {
		panic("endpoints: invalid domain" + domain)
	}
	return oauth2.Endpoint{
		AuthURL:  u.JoinPath("/oauth2/authorize").String(),
		TokenURL: u.JoinPath("/oauth2/token").String(),
	}
}

// HipChatServer returns a new oauth2.Endpoint for a HipChat Server instance.
// host should be a hostname, without any scheme prefix.
//
// Documentation: https://developer.atlassian.com/server/hipchat/hipchat-rest-api-access-tokens/
func HipChatServer(host string) oauth2.Endpoint {
	u := url.URL{
		Scheme: "https",
		Host:   host,
	}
	return oauth2.Endpoint{
		AuthURL:  u.JoinPath("/users/authorize").String(),
		TokenURL: u.JoinPath("/v2/oauth/token").String(),
	}
}

// Shopify returns a new oauth2.Endpoint for the supplied shop domain name.
// host should be a hostname, without any scheme prefix.
//
// Documentation: https://shopify.dev/docs/apps/auth/oauth
func Shopify(host string) oauth2.Endpoint {
	u := url.URL{
		Scheme: "https",
		Host:   host,
	}
	return oauth2.Endpoint{
		AuthURL:  u.JoinPath("/admin/oauth/authorize").String(),
		TokenURL: u.JoinPath("/admin/oauth/access_token").String(),
	}
}
