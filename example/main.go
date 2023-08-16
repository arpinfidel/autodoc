package main

import (
	"github.com/gin-gonic/gin"
)

func main() {

}

type ExampleRequest struct {
	ID          string `json:"id,omitempty" form:"id"`
	Name        string `json:"name,omitempty" form:"name"`
	Description string `json:"description,omitempty" form:"description"`
}

func ExampleFormHandler(statusCode int, resp interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(statusCode, resp)
	}
}
