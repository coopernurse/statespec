package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/coopernurse/statespec"
)

// Spec to test a Real World backend API server

func main() {
	iter := flag.Int("n", 100, "number of iterations to run")
	seed := flag.Int64("s", 0, "seed to use for RNG")
	endpoint := flag.String("e", "http://127.0.0.1:8585/api", "base url of endpoint to test")
	flag.Parse()

	if *seed == 0 {
		*seed = time.Now().UnixNano()
	}

	fmt.Printf("realworld api test. running %d iterations using seed %d against endpoint %s\n",
		*iter, *seed, *endpoint)
	gofakeit.Seed(*seed)
	conf := statespec.SpecConf{
		Rand:       rand.New(rand.NewSource(*seed)),
		Iterations: *iter,
	}
	iterRan, err := newRealWorldSpec(*endpoint).Run(conf)
	if err != nil {
		panic(err)
	}
	fmt.Printf("spec ok - %d iterations\n", iterRan)
}

func newRealWorldSpec(endpoint string) statespec.Spec[RealWorldState] {
	return statespec.Spec[RealWorldState]{
		InitState: func() RealWorldState {
			return RealWorldState{
				endpoint: endpoint,
			}
		},
		Commands: []statespec.Command[RealWorldState]{
			createUser,
			getCurrentUser,
			login,
		},
	}
}

type UserResponse struct {
	User User `json:"user"`
}

type User struct {
	Email    string `json:"email"`
	Token    string `json:"token"`
	Username string `json:"username"`
	Bio      string `json:"bio"`
	Image    string `json:"image"`
}

type NewUserRequest struct {
	NewUser NewUser `json:"user"`
}

type NewUser struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginUserRequest struct {
	LoginUser LoginUser `json:"user"`
}

type LoginUser struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RealWorldState struct {
	endpoint    string
	authToken   string
	password    string
	createUser  User
	loginUser   User
	currentUser User
}

func randNewUser() NewUser {
	return NewUser{
		Username: gofakeit.Username(),
		Password: gofakeit.Password(true, true, true, false, false, 2),
		Email:    gofakeit.Email(),
	}
}

func doPOST(u string, authToken string, input any, out any) error {
	return doHTTP("POST", u, authToken, input, out)
}

func doGET(u string, authToken string, out any) error {
	return doHTTP("GET", u, authToken, nil, out)
}

func doHTTP(method string, u string, authToken string, input any, out any) error {
	var inreader io.Reader
	if input != nil {
		injson, err := json.Marshal(input)
		if err != nil {
			return err
		}
		inreader = bytes.NewBuffer(injson)
	}

	client := &http.Client{}
	req, err := http.NewRequest(method, u, inreader)
	if err != nil {
		return err
	}
	if inreader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authToken != "" {
		req.Header.Set("Authorization", "Token "+authToken)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("doHTTP %s %s status %d", method, u, resp.StatusCode)
	}
	return json.Unmarshal(body, out)
}

var createUser = statespec.Command[RealWorldState]{
	Name: "createUser",
	Gen: func(state RealWorldState, rnd *rand.Rand) statespec.CommandFunc[RealWorldState] {
		input := NewUserRequest{NewUser: randNewUser()}
		return func() statespec.CommandOutput[RealWorldState] {
			var resp UserResponse
			state.password = ""
			err := doPOST(state.endpoint+"/users", state.authToken, input, &resp)
			if err == nil {
				state.createUser = resp.User
				state.password = input.NewUser.Password
			}
			return statespec.CommandOutput[RealWorldState]{NewState: state, Description: input, Error: err}
		}
	},
	Verify: func(oldState RealWorldState, newState RealWorldState) bool {
		return newState.createUser.Username != "" && newState.password != ""
	},
}

var getCurrentUser = statespec.Command[RealWorldState]{
	Name: "getCurrentUser",
	Gen: func(state RealWorldState, rnd *rand.Rand) statespec.CommandFunc[RealWorldState] {
		if state.authToken == "" {
			return nil
		}
		return func() statespec.CommandOutput[RealWorldState] {
			var resp UserResponse
			state.currentUser.Username = ""
			err := doGET(state.endpoint+"/user", state.authToken, &resp)
			if err == nil {
				state.currentUser = resp.User
			}
			return statespec.CommandOutput[RealWorldState]{NewState: state, Description: state.authToken, Error: err}

		}
	},
	Verify: func(oldState RealWorldState, newState RealWorldState) bool {
		return oldState.loginUser.Username == newState.currentUser.Username
	},
}

var login = statespec.Command[RealWorldState]{
	Name: "login",
	Gen: func(state RealWorldState, rnd *rand.Rand) statespec.CommandFunc[RealWorldState] {
		if state.createUser.Username == "" {
			return nil
		}
		input := LoginUserRequest{LoginUser: LoginUser{Email: state.createUser.Email, Password: state.password}}
		return func() statespec.CommandOutput[RealWorldState] {
			var resp UserResponse
			state.authToken = ""
			err := doPOST(state.endpoint+"/users/login", state.authToken, input, &resp)
			if err == nil {
				state.loginUser = resp.User
				state.authToken = resp.User.Token
			}
			return statespec.CommandOutput[RealWorldState]{NewState: state, Description: input, Error: err}
		}
	},
	Verify: func(oldState RealWorldState, newState RealWorldState) bool {
		return newState.loginUser.Username == newState.createUser.Username && newState.authToken != ""
	},
}
