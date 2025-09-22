// pkg/api/response.go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
)

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func WriteJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// If we can't encode the response, write a basic error
		http.Error(w, `{"success":false,"error":"Internal server error"}`, 
			http.StatusInternalServerError)
	}
}

// Standardized response helpers

func WriteSuccessResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	response := Response{
		Success: true,
		Data:    data,
	}
	WriteJSONResponse(w, statusCode, response)
}

func WriteErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	response := Response{
		Success: false,
		Error:   message,
	}
	WriteJSONResponse(w, statusCode, response)
}

func WriteBadRequestResponse(w http.ResponseWriter, message string) {
	WriteErrorResponse(w, http.StatusBadRequest, message)
}

func WriteUnauthorizedResponse(w http.ResponseWriter, message string) {
	WriteErrorResponse(w, http.StatusUnauthorized, message)
}

func WriteForbiddenResponse(w http.ResponseWriter, message string) {
	WriteErrorResponse(w, http.StatusForbidden, message)
}

func WriteNotFoundResponse(w http.ResponseWriter, message string) {
	WriteErrorResponse(w, http.StatusNotFound, message)
}

func WriteConflictResponse(w http.ResponseWriter, message string) {
	WriteErrorResponse(w, http.StatusConflict, message)
}

func WriteInternalErrorResponse(w http.ResponseWriter, message string) {
	WriteErrorResponse(w, http.StatusInternalServerError, message)
}

func WriteValidationErrorResponse(w http.ResponseWriter, err error) {
	validationErrors := make(map[string]string)
	
	if validationErrs, ok := err.(validator.ValidationErrors); ok {
		for _, fieldError := range validationErrs {
			field := fieldError.Field()
			tag := fieldError.Tag()
			
			switch tag {
			case "required":
				validationErrors[field] = field + " is required"
			case "email":
				validationErrors[field] = field + " must be a valid email"
			case "min":
				validationErrors[field] = field + " must be at least " + fieldError.Param() + " characters"
			case "max":
				validationErrors[field] = field + " must be at most " + fieldError.Param() + " characters"
			case "len":
				validationErrors[field] = field + " must be exactly " + fieldError.Param() + " characters"
			case "oneof":
				validationErrors[field] = field + " must be one of: " + fieldError.Param()
			default:
				validationErrors[field] = field + " is invalid"
			}
		}
	}

	response := Response{
		Success: false,
		Error:   "validation failed",
		Data:    map[string]interface{}{"validation_errors": validationErrors},
	}
	
	WriteJSONResponse(w, http.StatusBadRequest, response)
}