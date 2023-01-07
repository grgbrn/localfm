package web

import (
	"log"
	"net/http"
	"os"

	"golang.org/x/crypto/bcrypt"
)

// checks if email/passwd are valid login credentials
// returns (userId, nil) on successful login, where userid > 0
// returns (-1,nil) if credentials don't match
// err is only returned for internal errors
func authenticateUser(email, passwd string) (int, error) {

	// single user's login credentials are in env vars
	// passwd must be a bcrypt hashed string, 60 chars long
	// generate one with:
	//
	// hash, err := bcrypt.GenerateFromPassword([]byte("my plain text password"), 12)
	//
	// or:
	//
	// htpasswd -bnBC 12 "" password | tr -d ':\n'
	const userID = 100
	envLogin := os.Getenv("LOGIN_USER")
	if envLogin == "" {
		log.Println("Warning: LOGIN_USER environment var is not set")
	}
	envPass := os.Getenv("LOGIN_PASSWD")
	if envPass == "" {
		log.Println("Warning: LOGIN_PASSWD environment var is not set")
	}

	if envLogin != email {
		return -1, nil
	}

	hashedPassword := []byte(envPass)

	// Check whether the hashed password and plain-text password provided match
	err := bcrypt.CompareHashAndPassword(hashedPassword, []byte(passwd))
	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return -1, nil
		}
		return 0, err
	}
	// Otherwise, the password is correct. Return the user ID.
	return userID, nil
}

func (app *Application) isAuthenticated(r *http.Request) bool {
	return app.session.Exists(r, "authenticatedUserID")
}
