package cmd

import (
	"context"
	"fmt"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"log"
	"net/http"
	"net/url"
	"qovery.go/api"
	"qovery.go/util"
  "strconv"
	"time"
)

const (
	httpAuthPort   = 10999
	oAuthQoveryUrl = "https://auth.qovery.com/login?client=%s&protocol=oauth2&response_type=%s&audience=%s&redirect_uri=%s"
)

var (
	oAuthUrlParamValueClient    = "MJ2SJpu12PxIzgmc5z5Y7N8m5MnaF7Y0"
	oAuthUrlParamValueAudience  = "https://core.qovery.com"
	oAuthParamValueResponseType = "id_token token"
	oAuthUrlParamValueRedirect  = "http://localhost:" + strconv.Itoa(httpAuthPort) + "/authorization"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Do authentication",
	Long: `AUTH do auth on Qovery service. For example:

	qovery auth`,
	Run: func(cmd *cobra.Command, args []string) {
		available, message, _ := util.CheckAvailableNewVersion()
		if available {
			fmt.Println(message)
		}

		// TODO link to web auth
		_ = browser.OpenURL(fmt.Sprintf(oAuthQoveryUrl, oAuthUrlParamValueClient, url.QueryEscape(oAuthParamValueResponseType),
			url.QueryEscape(oAuthUrlParamValueAudience), url.QueryEscape(oAuthUrlParamValueRedirect)))

		fmt.Println("\nOpening your browser, waiting for your authentication...")

		srv := &http.Server{Addr: fmt.Sprintf("localhost:%d", httpAuthPort)}

		http.HandleFunc("/authorization", func(writer http.ResponseWriter, request *http.Request) {
			js := fmt.Sprintf(`<script type="text/javascript" charset="utf-8">
				var hash = window.location.hash.split("=")[1].split("&")[0];
				var xmlHttp = new XMLHttpRequest();
				xmlHttp.open("GET", "http://localhost:%d/authorization/valid?access_token=" + hash, false);
				xmlHttp.send(null);
				xmlHttp.responseText;
             </script>`, httpAuthPort)

			_, _ = writer.Write([]byte(js))
			_, _ = writer.Write([]byte("Authentication successful. You can close this window."))
		})

		http.HandleFunc("/authorization/valid", func(writer http.ResponseWriter, request *http.Request) {

			accessToken := request.URL.Query()["access_token"][0]

			api.SetAuthorizationToken(accessToken)

			accountId := api.GetAccount().Id
			if accountId != "" {
				api.SetAccountId(accountId)
				fmt.Println("Authentication successful!")
			}

			go func() {
				time.Sleep(time.Second)
				if err := srv.Shutdown(context.TODO()); err != nil {
					log.Printf("fail to shudown http server: %s", err.Error())
				}
			}()
		})

		srv.ListenAndServe()
	},
}

func init() {
	RootCmd.AddCommand(authCmd)
}
