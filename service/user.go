package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/olivere/elastic"
)

const (
	USER_INDEX = "user"
	USER_TYPE  = "user"
)

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Age      int64  `json:"age"`
	Gender   string `json:"gender"`
}

var signinKey = []byte("secret")

func handlerLogin(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received one login request")
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method == "OPTIONS" {
		return
	}

	decoder := json.NewDecoder(r.Body)
	var user User
	if err := decoder.Decode(&user); err != nil {
		http.Error(w, "Cannot decode user data from client", http.StatusBadRequest)
		fmt.Printf("Cannot decode user data from client %v.\n", err)
		return
	}

	if err := checkUser(user.Username, user.Password); err != nil {
		if err.Error() == "Incorrect username or password" {
			http.Error(w, "Incorrect username or password", http.StatusUnauthorized)
		} else {
			http.Error(w, "Failed to read from ElasticSearch", http.StatusInternalServerError)
		}
		return
	}

	// Create a new token object, specifying signing method and the claims
	// you would like it to contain.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": user.Username,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	})

	// Sign and get the complete encoded token as a string using the secret
	tokenString, err := token.SignedString(signinKey)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		fmt.Printf("Failed to generate token %v.\n", err)
		return
	}
	// If everything is correct, write the user into the result
	w.Write([]byte(tokenString))
}

func handlerSignup(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received one signup request")
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method == "OPTIONS" {
		return
	}

	decoder := json.NewDecoder(r.Body)
	var user User
	if err := decoder.Decode(&user); err != nil {
		http.Error(w, "Cannot decode user data from client", http.StatusBadRequest)
		fmt.Printf("Cannot decode user data from client %v.\n", err)
		return
	}

	// Check if the user is valid
	if user.Username == "" || user.Password == "" || !regexp.MustCompile(`^[a-z0-9_]+$`).MatchString(user.Username) {
		http.Error(w, "Invalid username or password", http.StatusBadRequest)
		fmt.Printf("Invalid username or password.\n")
		return
	}

	if err := addUser(user); err != nil {
		if err.Error() == "User already exists" {
			http.Error(w, "User already exists", http.StatusBadRequest)
		} else {
			http.Error(w, "Failed to save to ElasticSearch", http.StatusInternalServerError)
		}
		return
	}

	// If everything goes on well, write to the result
	w.Write([]byte("User created successfully"))
}

func checkUser(username, password string) error {
	// Create an ElasticSearch client (connection)
	client, err := elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false))
	if err != nil {
		return err
	}

	// Create a query statement
	// SELECT * FROM users WHERE username = ?
	query := elastic.NewTermQuery("username", username)
	// Get the search result from the query statement
	searchResult, err := client.Search().
		Index(USER_INDEX).
		Query(query).
		Pretty(true).
		Do(context.Background())
	if err != nil {
		return err
	}

	// Iterate over the query result and look for the correct user
	var utyp User
	for _, item := range searchResult.Each(reflect.TypeOf(utyp)) {
		if u, ok := item.(User); ok {
			// Check if the user's login authorization is correct
			if username == u.Username && password == u.Password {
				fmt.Printf("Login as %s\n", username)
				return nil
			}
		}
	}

	return errors.New("Incorrect username or password")
}

func addUser(user User) error {
	// Create an ElasticSearch client (connection)
	client, err := elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false))
	if err != nil {
		return err
	}

	// Create a query statement
	// SELECT * FROM users WHERE username = ?
	query := elastic.NewTermQuery("username", user.Username)
	// Get the search result from the query statement
	searchResult, err := client.Search().
		Index(USER_INDEX).
		Query(query).
		Pretty(true).
		Do(context.Background())
	if err != nil {
		return err
	}

	// Check if the user has been created already
	if searchResult.TotalHits() > 0 {
		return errors.New("User already exists")
	}

	// Insert the user
	_, err = client.Index().
		Index(USER_INDEX).
		Type(USER_TYPE).
		Id(user.Username).
		BodyJson(user).
		Refresh("wait_for").
		Do(context.Background())
	if err != nil {
		return err
	}

	fmt.Printf("User is added: %s\n", user.Username)
	return nil
}
