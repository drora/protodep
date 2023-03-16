package session

import (
	"errors"
	"fmt"
	"github.com/manifoldco/promptui"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
)

type Session interface {
	Login() error
	Logout() error
	GetUser() string
	GetToken() string
}

type session struct {
	conf  *Config
	User  string
	Token string
}

const sessionDataDelimiter = ":"

var protodepConfigFile string

func New(conf *Config) Session {
	s := &session{
		conf:  conf,
		User:  "",
		Token: "",
	}
	protodepConfigFile = filepath.Join(s.conf.HomeDir, ".protodep/config")

	if HasSession() {
		sessionData, err := ReadSessionData()
		if err != nil {
			fmt.Printf("invalid session detected, please login: %v\n", err)
		}
		if len(sessionData) == 0 {
			fmt.Printf("session not found or empty, please login.")
		}
		sessionSplit := strings.Split(sessionData, sessionDataDelimiter)
		if len(sessionSplit) != 2 {
			fmt.Printf("session corrupted, please login.")
		} else {
			s.User = sessionSplit[0]
			s.Token = sessionSplit[1]
		}
	}
	return s
}

func (s *session) GetUser() string {
	return s.User
}

func (s *session) GetToken() string {
	return s.Token
}

func (s *session) Login() error {
	fmt.Println("Logging in...")
	username := promptUser()
	if len(username) == 0 {
		return errors.New("user not provided, input aborted")
	}
	token := promptToken()
	if len(token) == 0 {
		return errors.New("token not provided, input aborted")
	}
	content := username + sessionDataDelimiter + token
	if err := WriteSessionData(content); err != nil {
		return err
	}
	fmt.Println("OK")
	return nil
}

func (s *session) Logout() error {
	fmt.Println("Logging out...")
	if HasSession() {
		if err := os.Remove(protodepConfigFile); err != nil {
			return err
		}
		fmt.Println("Bye!")
	} else {
		fmt.Printf("session not found")
	}
	return nil
}

func promptUser() string {
	var username string
	u, err := user.Current()
	if err == nil {
		username = u.Username
	}
	prompt := promptui.Prompt{
		Label:     "what's your github user?",
		Validate:  validateUser,
		Default:   username,
		AllowEdit: true,
	}

	result, err := prompt.Run()
	if err != nil {
		fmt.Printf("failed reading user: %v\n", err)
		return ""
	}
	return result
}

func promptToken() string {
	fmt.Printf(`
a personal access token is required to allow protodep access to dependency sources.

generate your personal token here: https://github.com/settings/tokens
- make sure you enable the 'repo' scope so your token will have READ privileges to private repositories.
- once token is created and copied, make sure you configure SSO and authorize access to needed private organizations.

read more about personal access tokens: https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token
`)
	prompt := promptui.Prompt{
		Label:    "personal access token",
		Validate: validateToken,
		Mask:     '*',
	}

	result, err := prompt.Run()
	if err != nil {
		fmt.Printf("failed reading token: %v\n", err)
		return ""
	}
	return result
}

func validateUser(input string) error {
	if len(input) < 2 {
		return errors.New("username must have more than 2 characters")
	}
	return nil
}

func validateToken(input string) error {
	var githubTokenPattern string = "^(ghp_[a-zA-Z0-9]{36}|github_pat_[a-zA-Z0-9]{22}_[a-zA-Z0-9]{59}|v[0-9]\\.[0-9a-f]{40})$"
	_, err := regexp.MatchString(githubTokenPattern, input)
	return err
}

func HasSession() bool {
	if _, err := os.Stat(protodepConfigFile); err == nil {
		return true
	}
	return false
}

func ReadSessionData() (string, error) {
	content, err := os.ReadFile(protodepConfigFile)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

func WriteSessionData(content string) error {
	f, createErr := os.Create(protodepConfigFile)
	if createErr != nil {
		return createErr
	}
	defer f.Close()

	_, writeErr := f.WriteString(content + "\n")
	if writeErr != nil {
		return writeErr
	}
	return nil
}
