package response

import "net/http"

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type PaginatedResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Meta    Meta        `json:"meta"`
}

type Meta struct {
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalPages int   `json:"total_pages"`
}

func OK(c interface{ JSON(int, interface{}) }, message string, data interface{}) {
	c.JSON(http.StatusOK, ApiResponse{Success: true, Message: message, Data: data})
}

func Created(c interface{ JSON(int, interface{}) }, message string, data interface{}) {
	c.JSON(http.StatusCreated, ApiResponse{Success: true, Message: message, Data: data})
}

func BadRequest(c interface{ JSON(int, interface{}) }, message string) {
	c.JSON(http.StatusBadRequest, ApiResponse{Success: false, Message: message})
}

func Unauthorized(c interface{ JSON(int, interface{}) }, message string) {
	c.JSON(http.StatusUnauthorized, ApiResponse{Success: false, Message: message})
}

func Forbidden(c interface{ JSON(int, interface{}) }, message string) {
	c.JSON(http.StatusForbidden, ApiResponse{Success: false, Message: message})
}

func NotFound(c interface{ JSON(int, interface{}) }, message string) {
	c.JSON(http.StatusNotFound, ApiResponse{Success: false, Message: message})
}

func InternalError(c interface{ JSON(int, interface{}) }, message string) {
	c.JSON(http.StatusInternalServerError, ApiResponse{Success: false, Message: message})
}

func Paginated(c interface{ JSON(int, interface{}) }, message string, data interface{}, meta Meta) {
	c.JSON(http.StatusOK, PaginatedResponse{Success: true, Message: message, Data: data, Meta: meta})
}

func CalcTotalPages(total int64, limit int) int {
	if limit == 0 {
		return 0
	}
	pages := int(total) / limit
	if int(total)%limit != 0 {
		pages++
	}
	return pages
}
