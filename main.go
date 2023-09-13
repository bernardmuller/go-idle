package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB

type User struct {
	gorm.Model
	ID        uint    `gorm:"primaryKey" json:"id"`
	Name      string  `                  json:"name"`
	Email     *string `gorm:"unique"     json:"email"`
	Password  string  `                  json:"password"`
	role      Role
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Role struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Name        string `                  json:"name"`
	Permissions []Permission
}

type Permission struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `                  json:"name"`
}

type RegisterDTO struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type LoginDTO struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type SuccessResponse[T any] struct {
	StatusCode int `json:"status"`
	Data       T   `json:"data"`
}

type ErrorResponse struct {
	StatusCode   int    `json:"status"`
	ErrorMessage string `json:"message"`
}

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type Token struct {
	Token string `json:"token"`
}

var JWT_KEY = []byte("my_secret_key")

func setupEnv() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

// Middleware
func logging(f httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		log.Println(r.URL.Path)
		f(w, r, p)
	}
}

func authenticate(f httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		request_token := r.Header.Get("Authorization")
		if request_token == "" {
			error := ErrorResponse{
				StatusCode:   http.StatusUnauthorized,
				ErrorMessage: "Unauthorized",
			}
			json.NewEncoder(w).Encode(error)
			return
		}

		f(w, r, p)
	}
}

// Handlers
func Index(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	user := User{}
	db.First(&user, 1)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	json.NewEncoder(w).Encode(user)
}

func Register(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	dto := RegisterDTO{}
	json.NewDecoder(r.Body).Decode(&dto)
	if dto.Email == "" || dto.Password == "" || dto.Name == "" {
		error := ErrorResponse{
			StatusCode:   http.StatusBadRequest,
			ErrorMessage: "Invalid request payload",
		}
		json.NewEncoder(w).Encode(error)
		return
	}

	hashed_password, err := HashPassword(dto.Password)
	if err != nil {
		panic(err)
	}
	dto.Password = hashed_password

	db.Create(&User{Name: dto.Name, Email: &dto.Email, Password: dto.Password})
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	new_user := db.Find(&User{}).Where("email = ?", dto.Email)
	json.NewEncoder(w).Encode(new_user)
}

func Login(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	dto := LoginDTO{}
	json.NewDecoder(r.Body).Decode(&dto)
	if dto.Email == "" || dto.Password == "" {
		error := ErrorResponse{
			StatusCode:   http.StatusBadRequest,
			ErrorMessage: "Invalid request payload",
		}
		json.NewEncoder(w).Encode(error)
		return
	}

	var user User
	db.Where("email = ?", dto.Email).First(&user)
	if &user == nil {
		error := ErrorResponse{
			StatusCode:   http.StatusUnauthorized,
			ErrorMessage: "Invalid credentials",
		}
		json.NewEncoder(w).Encode(error)
		return
	}

	valid_password := CheckPasswordHash(dto.Password, user.Password)
	if !valid_password {
		error := ErrorResponse{
			StatusCode:   http.StatusUnauthorized,
			ErrorMessage: "Invalid credentials",
		}
		json.NewEncoder(w).Encode(error)
		return
	}

	expirationTime := time.Now().Add(5 * time.Minute)

	claims := &Claims{
		Username: user.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(JWT_KEY)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	login_response := SuccessResponse[Token]{
		StatusCode: http.StatusOK,
		Data: Token{
			Token: tokenString,
		},
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	json.NewEncoder(w).Encode(login_response)
}

func getUsers(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	var users []User
	db.Find(&users)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	json.NewEncoder(w).Encode(users)
}

func deleteUser(w http.ResponseWriter, _ *http.Request, p httprouter.Params) {
	id := p.ByName("id")
	var user User
	db.First(&user, id)
	db.Delete(&user)
	var users []User
	db.Find(&users)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	json.NewEncoder(w).Encode(users)
}

// Utils
func getEnvVar(key string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		log.Fatalf("Environment variable %s not set.", key)
	}
	return value
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func main() {
	setupEnv()
	router := httprouter.New()

	dbUrl := getEnvVar("DATABASE_URL")
	db_connection, err := gorm.Open(postgres.Open(dbUrl), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	db = db_connection
	db.AutoMigrate(&User{})

	router.GET("/user", logging(Index))
	router.POST("/register", Register)
	router.POST("/login", Login)
	router.GET("/users", getUsers)
	router.DELETE("/users/:id", deleteUser)

	log.Fatal(http.ListenAndServe(":8080", router))
}
