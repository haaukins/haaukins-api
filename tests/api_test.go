package tests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gorilla/websocket"

	"github.com/rs/zerolog"

	"github.com/aau-network-security/haaukins-api/app"
)

const (
	whatever            = "whatever"
	requestedChallenges = "challenges"
	sessionCookie       = "haaukins_session"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}

func getTestConfig(totalR, clientR int) *app.Config {
	return &app.Config{
		Host: "localhost",
		Port: struct {
			Secure   uint `yaml:"secure,omitempty"`
			InSecure uint `yaml:"insecure,omitempty"`
		}{
			Secure:   443,
			InSecure: 80,
		},
		TLS:           app.CertificateConfig{},
		ExercisesFile: "",
		API: app.APIConfig{
			SignKey: whatever,
			Admin: struct {
				Username string `yaml:"username"`
				Password string `yaml:"password"`
			}{
				Username: whatever,
				Password: whatever,
			},
			Captcha: struct {
				Enabled   bool   `yaml:"enabled"`
				SiteKey   string `yaml:"site-key"`
				SecretKey string `yaml:"secret-key"`
			}{Enabled: false},
			TotalMaxRequest:  totalR,
			ClientMaxRequest: clientR,
		},
	}
}

//Test requests made to the API
func TestCorrectRequests(t *testing.T) {

	config := getTestConfig(10, 4)

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting the working directory: %s", err.Error())
	}
	config.ExercisesFile = dir + "/exercises_test.yml"

	lm, err := app.New(config, true)

	if err != nil {
		t.Fatalf("Error Creating API : %s", err.Error())
	}

	ts := httptest.NewServer(lm.Handler())

	tt := []struct {
		name       string
		request    string
		statusCode int
	}{
		{name: "Normal", request: fmt.Sprintf("%s/api/?%s=xxxx", ts.URL, requestedChallenges), statusCode: http.StatusServiceUnavailable},
		{name: "Bad Request", request: fmt.Sprintf("%s/api/?whatever=xxxx", ts.URL), statusCode: http.StatusBadRequest},
		{name: "Wrong Exercise Tag", request: fmt.Sprintf("%s/api/?challenges=whatever", ts.URL), statusCode: http.StatusBadRequest},
		{name: "Wrong Path", request: fmt.Sprintf("%s/whatever/", ts.URL), statusCode: http.StatusNotFound},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			req, err := http.NewRequest("GET", tc.request, nil)
			if err != nil {
				t.Fatalf("Error building request: %s", err.Error())
			}
			client := &http.Client{
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Error getting response: %s", err.Error())
			}
			if resp.StatusCode != tc.statusCode {
				t.Fatalf("Status code Error. Expected [%d], got [%d]", tc.statusCode, resp.StatusCode)
			}
		})
	}
}

//Test request made to the admin part
func TestAdminRequests(t *testing.T) {
	config := getTestConfig(10, 4)

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting the working directory: %s", err.Error())
	}
	config.ExercisesFile = dir + "/exercises_test.yml"

	lm, err := app.New(config, true)

	if err != nil {
		t.Fatalf("Error Creating API : %s", err.Error())
	}

	ts := httptest.NewServer(lm.Handler())

	//Create a new client request
	cr := lm.NewClient("localhost")
	_ = cr.NewClientRequest("yyyy")

	tt := []struct {
		name     string
		allowed  bool
		username string
		password string
		code     int
	}{
		{name: "Not Allowed Client", allowed: true, username: "test", password: "test", code: http.StatusUnauthorized},
		{name: "Not Allowed Client2", allowed: false, code: http.StatusUnauthorized},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			req, err := http.NewRequest("GET", fmt.Sprintf("%s/admin/envs/", ts.URL), nil)
			if err != nil {
				t.Fatalf("Error building request: %s", err.Error())
			}
			if tc.allowed {
				req.SetBasicAuth(tc.username, tc.password)
			}
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Error getting response: %s", err.Error())
			}
			if resp.StatusCode != tc.code {
				t.Fatalf("Status code Error. Expected [%d], got [%d]", tc.code, resp.StatusCode)
			}
		})
	}
}

//Test number of requests made by a client
func TestClientRequests(t *testing.T) {
	//The client can make just a request
	config := getTestConfig(10, 1)

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting the working directory: %s", err.Error())
	}
	config.ExercisesFile = dir + "/exercises_test.yml"

	lm, err := app.New(config, true)
	if err != nil {
		t.Fatalf("Error Creating API : %s", err.Error())
	}

	ts := httptest.NewServer(lm.Handler())

	//the client make a request here
	cr := lm.NewClient("localhost")
	token, err := cr.CreateToken(config.API.SignKey)
	if err != nil {
		t.Fatalf("Error creating Token: %s", err.Error())
	}
	_ = cr.NewClientRequest("yyyy")

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/?%s=xxxx", ts.URL, requestedChallenges), nil)
	if err != nil {
		t.Fatalf("Error building request: %s", err.Error())
	}

	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token, Path: "/"})
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Error getting response: %s", err.Error())
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("Status code Error. Expected [%d], got [%d]", http.StatusTooManyRequests, resp.StatusCode)
	}
}

//Test total number of request the API can handle
func TestAPIRequests(t *testing.T) {
	//The API can handle 5 requests max
	config := getTestConfig(5, 2)

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting the working directory: %s", err.Error())
	}
	config.ExercisesFile = dir + "/exercises_test.yml"

	lm, err := app.New(config, true)
	if err != nil {
		t.Fatalf("Error Creating API : %s", err.Error())
	}

	ts := httptest.NewServer(lm.Handler())

	tt := []struct {
		name     string
		requests []string
	}{
		{name: "Client1", requests: []string{"xxxx", "yyyy"}},
		{name: "Client2", requests: []string{"xxxx", "yyyy"}},
		{name: "Client3", requests: []string{"xxxx", "yyyy"}},
	}

	for _, tc := range tt {
		cr := lm.NewClient(tc.name)
		for _, r := range tc.requests {
			_ = cr.NewClientRequest(r)
		}
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/?%s=xxxx", ts.URL, requestedChallenges), nil)
	if err != nil {
		t.Fatalf("Error building request: %s", err.Error())
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Error getting response: %s", err.Error())
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("Status code Error. Expected [%d], got [%d]", http.StatusServiceUnavailable, resp.StatusCode)
	}
}

func TestFrontendRequest(t *testing.T) {
	config := getTestConfig(5, 2)

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting the working directory: %s", err.Error())
	}
	config.ExercisesFile = dir + "/exercises_test.yml"

	lm, err := app.New(config, true)
	if err != nil {
		t.Fatalf("Error Creating API : %s", err.Error())
	}

	ts := httptest.NewServer(lm.Handler())

	wsUrl := fmt.Sprintf("%s/challengesFrontend", strings.Replace(ts.URL, "http", "ws", 1))
	c, _, err := websocket.DefaultDialer.Dial(wsUrl, nil)
	if err != nil {
		t.Fatalf("Error WS: %s", err.Error())
	}

	_, resp, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("Error getting challenges from frontend: %s", err.Error())
	}

	if `{"msg":"challenges_categories","values":null}` != string(resp) {
		t.Fatal("WS response doesn't match")
	}
	c.Close()
}
