package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ustasjs/gopher-markt/internal/service"
	"github.com/ustasjs/gopher-markt/internal/storage"
)

type UserHandler struct {
	userService service.UserServiceInterface
}

func NewUserHandler(userService service.UserServiceInterface) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

type RegisterRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Login == "" || req.Password == "" {
		http.Error(w, "login and password are required", http.StatusBadRequest)
		return
	}

	token, err := h.userService.Register(r.Context(), req.Login, req.Password)
	if err != nil {
		// Handle login conflict
		if errors.Is(err, storage.ErrLoginConflict) {
			http.Error(w, "login already exists", http.StatusConflict)
			return
		}

		// Handle other errors
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Authorization", "Bearer "+token)
	w.WriteHeader(http.StatusOK)
}

func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Login == "" || req.Password == "" {
		http.Error(w, "login and password are required", http.StatusBadRequest)
		return
	}

	token, err := h.userService.Login(r.Context(), req.Login, req.Password)
	if err != nil {
		// Handle user not found or invalid password
		if errors.Is(err, storage.ErrUserNotFound) || err.Error() == "invalid password" {
			http.Error(w, "invalid login or password", http.StatusUnauthorized)
			return
		}

		// Handle other errors
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Authorization", "Bearer "+token)
	w.WriteHeader(http.StatusOK)
}
