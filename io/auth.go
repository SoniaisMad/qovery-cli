package io

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pkg/browser"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	httpAuthPort   = 10999
	oAuthQoveryUrl = "https://auth.qovery.com/login?code_challenge_method=S256&scope=%s&client=%s&protocol=oauth2&response_type=%s&audience=%s&redirect_uri=%s&code_challenge=%s"
)

var (
	oAuthUrlParamValueClient         = "MJ2SJpu12PxIzgmc5z5Y7N8m5MnaF7Y0"
	oAuthUrlParamValueHeadlessClient = "f9drkTNpxsEw2VU2PVDrxhyT3vVuFT0Y"
	oAuthUrlParamValueAudience       = "https://core.qovery.com"
	oAuthUrlParamValueResponseType   = "code"
	oAuthUrlParamValueScopes         = "offline_access openid profile email"
	oAuthUrlParamValueRedirect       = "http://localhost:" + strconv.Itoa(httpAuthPort) + "/authorization"
	oAuthTokenEndpoint               = "https://auth.qovery.com/oauth/token"
)

type TokensResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func DoRequestUserToAuthenticate(headless bool) {
	qoveryConsoleUrl := "https://console.qovery.com"

	available, message, _ := CheckAvailableNewVersion()
	if available {
		fmt.Println(message)
	}

	if headless {
		runHeadlessFlow()
	}

	verifier := createCodeVerifier()
	challenge, err := createCodeChallengeS256(verifier)
	if err != nil {
		fmt.Println("Can not create authorization code challenge. Please contact the #support at 'https://discord.qovery.com'.")
		os.Exit(1)
	}
	// TODO link to web auth
	_ = browser.OpenURL(fmt.Sprintf(oAuthQoveryUrl, url.QueryEscape(oAuthUrlParamValueScopes), oAuthUrlParamValueClient, url.QueryEscape(oAuthUrlParamValueResponseType),
		url.QueryEscape(oAuthUrlParamValueAudience), url.QueryEscape(oAuthUrlParamValueRedirect), challenge))

	fmt.Println("\nOpening your browser, waiting for your authentication...")

	srv := &http.Server{Addr: fmt.Sprintf("localhost:%d", httpAuthPort)}

	http.HandleFunc("/authorization", func(writer http.ResponseWriter, request *http.Request) {
		js := fmt.Sprintf(`<script type="text/javascript" charset="utf-8">
				var hash = window.location.search.split("=")[1].split("&")[0];
				var xmlHttp = new XMLHttpRequest();
				xmlHttp.open("GET", "http://localhost:%d/authorization/valid?code=" + hash, false);
				xmlHttp.send(null);
				xmlHttp.responseText;
				window.setTimeout('window.location="` + qoveryConsoleUrl + `"; ',2000);
             </script>`, httpAuthPort)

		_, _ = writer.Write([]byte(js))
		_, _ = writer.Write([]byte("Authentication successful, you'll be redirected to Qovery console. If it's not the case, click on this link: <a href='" + qoveryConsoleUrl + "'>" + qoveryConsoleUrl + "</a>"))
	})

	http.HandleFunc("/authorization/valid", func(writer http.ResponseWriter, request *http.Request) {
		code := request.URL.Query()["code"][0]
		res, err := http.PostForm(oAuthTokenEndpoint, url.Values{
			"grant_type":    {"authorization_code"},
			"client_id":     {oAuthUrlParamValueClient},
			"code":          {code},
			"redirect_uri":  {oAuthUrlParamValueRedirect},
			"code_verifier": {verifier},
		})

		if err != nil {
			println("Authentication unsuccessful. Try again later or contact #support on 'https://discord.qovery.com'. ")
			os.Exit(1)
		} else {
			defer res.Body.Close()
			tokens := TokensResponse{}
			err := json.NewDecoder(res.Body).Decode(&tokens)
			if err != nil {
				println("Authentication unsuccessful. Try again later or contact #support on 'https://discord.qovery.com'. ")
				os.Exit(1)
			}
			expiredAt := tokenExpiration()
			SetAuthorizationToken(tokens.AccessToken)
			SetRefreshToken(tokens.RefreshToken)
			SetAuthorizationTokenExpiration(expiredAt)
			accountId := GetAccount().Id
			if accountId != "" {
				SetAccountId(accountId)
				fmt.Println("Authentication successful!")
			}
		}

		go func() {
			time.Sleep(time.Second)
			if err := srv.Shutdown(context.TODO()); err != nil {
				log.Printf("fail to shudown http server: %s", err.Error())
			}
		}()
	})

	_ = srv.ListenAndServe()
}

func RefreshAccessToken() error {
	refreshToken := strings.TrimSpace(GetRefreshToken())
	if refreshToken == "" {
		return errors.New("Could not reauthenticate automatically. Please, run 'qovery auth' to authenticate. ")
	}
	res, err := http.PostForm(oAuthTokenEndpoint, url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {oAuthUrlParamValueClient},
		"refresh_token": {refreshToken},
	})
	if err != nil {
		return errors.New("Error authenticating in Qovery. Please, contact the #support on 'https://discord.qovery.com'. ")
	} else {
		defer res.Body.Close()
		tokens := TokensResponse{}
		err := json.NewDecoder(res.Body).Decode(&tokens)
		if err != nil {
			return errors.New("Error authenticating in Qovery. Please, contact the #support on 'https://discord.qovery.com'. ")
		}
		expiredAt := time.Now().Local().Add(time.Second * time.Duration(30000))
		SetAuthorizationToken(tokens.AccessToken)
		SetAuthorizationTokenExpiration(expiredAt)
		accountId := GetAccount().Id
		if accountId != "" {
			SetAccountId(accountId)
		} else {
			return errors.New("Could not reauthenticate automatically. Please, run 'qovery auth' to authenticate. ")
		}
	}
	return nil
}

func RefreshExpiredTokenSilently() {
	refreshToken := strings.TrimSpace(GetRefreshToken())
	expiration, err := GetAuthorizationTokenExpiration()

	if err == nil && expiration.Before(time.Now()) && refreshToken != "" {
		_ = RefreshAccessToken()
	}
}

func createCodeVerifier() string {
	length := 64
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := 0; i < length; i++ {
		b[i] = byte(r.Intn(255))
	}
	return encode(b)
}

func createCodeChallengeS256(verifier string) (string, error) {
	h := sha256.New()
	_, err := h.Write([]byte(verifier))
	if err != nil {
		return "", err
	}
	return encode(h.Sum(nil)), nil
}

func encode(msg []byte) string {
	encoded := base64.StdEncoding.EncodeToString(msg)
	encoded = strings.Replace(encoded, "+", "-", -1)
	encoded = strings.Replace(encoded, "/", "_", -1)
	encoded = strings.Replace(encoded, "=", "", -1)
	return encoded
}

func runHeadlessFlow() {
	parameters := deviceFlowParameters()
	requestDeviceActivationWith(parameters)
	start := time.Now()

	fmt.Println("Waiting for code confirmation...")

	for time.Since(start).Seconds() < float64(parameters.ExpiresIn) {
		time.Sleep(time.Second * time.Duration(parameters.Interval))
		tokens, err := getTokensWith(parameters)

		if err == nil {
			expiredAt := tokenExpiration()
			SetRefreshToken(tokens.RefreshToken)
			SetAuthorizationToken(tokens.AccessToken)
			SetAuthorizationTokenExpiration(expiredAt)
			accountId := GetAccount().Id
			if accountId != "" {
				SetAccountId(accountId)
				fmt.Println("Authentication successful!")
				os.Exit(0)
			} else {
				printContactSupportMessage("Could not create account identifier")
				os.Exit(1)
			}
		}
	}

	fmt.Println("Code has expired!")
	os.Exit(1)
}

func tokenExpiration() time.Time {
	return time.Now().Local().Add(time.Second * time.Duration(30000))
}

func deviceFlowParameters() DeviceFlowParameters {
	endpoint := "https://auth.qovery.com/oauth/device/code"
	payload := strings.NewReader(fmt.Sprintf("client_id=%s&scope=%s&audience=%s&redirect_uri=%s", url.QueryEscape(oAuthUrlParamValueHeadlessClient), url.QueryEscape(oAuthUrlParamValueScopes), url.QueryEscape(oAuthUrlParamValueAudience), url.QueryEscape(oAuthUrlParamValueRedirect)))
	req, err := http.NewRequest("POST", endpoint, payload)

	if err != nil {
		printContactSupportMessage("Error forming device code request.")
		os.Exit(1)
	}

	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)

	if err != nil {
		printContactSupportMessage("Error getting device code.")
		os.Exit(1)
	}

	if res.StatusCode == 200 {
		defer res.Body.Close()

		parameters := DeviceFlowParameters{}
		err = json.NewDecoder(res.Body).Decode(&parameters)

		if err != nil {
			printContactSupportMessage("Error parsing device code response.")
			os.Exit(1)
		}

		return parameters
	} else {
		printContactSupportMessage("Error getting device code.")
		os.Exit(1)
		return DeviceFlowParameters{}
	}
}

func printContactSupportMessage(msg string) {
	fmt.Println(msg)
	fmt.Println("Please contact the #support at 'https://discord.qovery.com'.")
}

func requestDeviceActivationWith(params DeviceFlowParameters) {
	fmt.Println("Please, open browser @ " + params.VerificationUri + " using any device and enter " + params.UserCode + " code.")
}

func getTokensWith(params DeviceFlowParameters) (TokensResponse, error) {
	endpoint := "https://auth.qovery.com/oauth/token"
	payload := strings.NewReader("grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Adevice_code&device_code=" + params.DeviceCode + "&client_id=" + oAuthUrlParamValueHeadlessClient)
	req, err := http.NewRequest("POST", endpoint, payload)

	if err != nil {
		printContactSupportMessage("Error forming get access token request.")
		os.Exit(1)
	}

	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)

	if err != nil {
		printContactSupportMessage("Error pooling access token.")
		os.Exit(1)
	}

	defer res.Body.Close()

	if res.StatusCode == 200 {
		tokens := TokensResponse{}
		err = json.NewDecoder(res.Body).Decode(&tokens)
		return tokens, err
	} else {
		return TokensResponse{}, errors.New("Could not fetch tokens")
	}
}

func GetTokens() (string, string) {
	authorizationToken := GetAuthorizationToken()
	if authorizationToken == "" {
		log.Fatal("Authorization token not found. Use 'qovery auth' to sign in first. ")
	}

	adminToken := getAdminToken(authorizationToken)
	if adminToken == "" {
		log.Fatal("Admin token not found. Use 'qovery auth' to sign as admin user first. ")
	}

	return authorizationToken, adminToken
}

func getAdminToken(authorizationToken string) string {
	type Response struct {
		AccessToken string `json:"access_token"`
	}

	req, _ := http.NewRequest(http.MethodGet, RootURL+"/admin/management-token", nil)
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(authorizationToken))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	var parsedResBody Response
	err = json.Unmarshal(body, &parsedResBody)
	if err != nil {
		log.Fatal(err)
	}

	return strings.TrimSpace(parsedResBody.AccessToken)
}

type DeviceFlowParameters struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationUri         string `json:"verification_uri"`
	VerificationUriComplete string `json:"verification_uri_complete"`
	ExpiresIn               int64  `json:"expires_in"`
	Interval                int64  `json:"interval"`
}
