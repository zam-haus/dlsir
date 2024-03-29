package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
)

func _log(c *gin.Context, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)

	if c != nil {
		log.Printf("[%v] %v", c.RemoteIP(), msg)
	} else {
		log.Printf("[--] %v", msg)
	}
}
