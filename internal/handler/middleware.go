package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func CheckAuthorization(c *gin.Context) {
	name, ok := c.Get("name")
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "check login or password",
		})
	}
	fmt.Print(name)
}
